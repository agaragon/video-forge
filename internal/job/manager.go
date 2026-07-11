package job

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/agaragon/video-forge/internal/media"
)

// Encoder is the subset of media.Encoder's behavior the job manager depends
// on. media.Encoder satisfies this interface structurally, which keeps this
// package testable with a fake encoder and free of a hard dependency on
// invoking a real ffmpeg binary.
type Encoder interface {
	Encode(ctx context.Context, inputPath, outputPath string, p media.Params, totalDurationS float64, onProgress func(media.Progress)) error
}

// ErrNotFound is returned when a job ID has no matching Job.
var ErrNotFound = fmt.Errorf("job: not found")

// ErrNotCancelable is returned when Cancel is called on a job that has
// already reached a terminal state.
var ErrNotCancelable = fmt.Errorf("job: not running or pending")

// Manager owns the job store, the pending queue, and the worker pool that
// drains it (NFR-2 concurrency, REQ-JOB-1..9).
type Manager struct {
	encoder         Encoder
	queue           Queue
	workerCount     int
	jobTimeout      time.Duration
	outputRetention time.Duration

	mu   sync.RWMutex
	jobs map[string]*Job

	wg sync.WaitGroup
}

// NewManager constructs a Manager. Call Start to launch its worker pool.
func NewManager(encoder Encoder, workerCount int, jobTimeout, outputRetention time.Duration) *Manager {
	return &Manager{
		encoder:         encoder,
		queue:           NewMemoryQueue(),
		workerCount:     workerCount,
		jobTimeout:      jobTimeout,
		outputRetention: outputRetention,
		jobs:            make(map[string]*Job),
	}
}

// Start launches the worker pool and the retention sweeper. It returns
// immediately; workers run until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	for i := 0; i < m.workerCount; i++ {
		m.wg.Add(1)
		go m.workerLoop(ctx)
	}
	m.wg.Add(1)
	go m.retentionSweeper(ctx)
}

// Wait blocks until all workers have exited (used by graceful shutdown).
func (m *Manager) Wait() {
	m.wg.Wait()
}

// Submit creates a new pending Job under the given id and enqueues it,
// implementing REQ-JOB-1 (enqueue + unique ID) and REQ-JOB-6 (report queue
// position). Callers generate id via NewID() beforehand when they need to
// know it ahead of time (e.g. to allocate the output file path).
func (m *Manager) Submit(id string, kind media.Kind, sourcePath, outputPath string, info media.Info, params media.Params) *Job {
	j := newJob(id, kind, sourcePath, outputPath, info, params)

	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()

	pos := m.queue.Push(id)
	j.setQueuePosition(pos)
	return j
}

// QueueLen returns the number of jobs currently waiting for a free worker
// (REQ-JOB-6, REQ-OPS-2).
func (m *Manager) QueueLen() int {
	return m.queue.Len()
}

// Get looks up a job by ID.
func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.jobs[id]
	return j, ok
}

// Cancel stops a running job's ffmpeg process, or short-circuits a pending
// one before it starts (REQ-JOB-3/4).
func (m *Manager) Cancel(id string) error {
	j, ok := m.Get(id)
	if !ok {
		return ErrNotFound
	}

	j.mu.Lock()
	switch j.state {
	case StatePending:
		j.state = StateCancelled
		now := time.Now().UTC()
		j.completedAt = now
		j.mu.Unlock()
		return nil
	case StateRunning:
		cancel := j.cancel
		j.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return nil
	default:
		j.mu.Unlock()
		return ErrNotCancelable
	}
}

func (m *Manager) workerLoop(ctx context.Context) {
	defer m.wg.Done()
	for {
		id, ok := m.queue.Pop(ctx)
		if !ok {
			return // ctx cancelled and queue drained
		}
		j, ok := m.Get(id)
		if !ok {
			continue
		}
		m.runJob(ctx, j)
	}
}

func (m *Manager) runJob(parent context.Context, j *Job) {
	j.mu.Lock()
	if j.state != StatePending {
		// Already cancelled while it sat in the queue.
		j.mu.Unlock()
		return
	}
	jobCtx, cancel := context.WithTimeout(parent, m.jobTimeout)
	j.state = StateRunning
	j.startedAt = time.Now().UTC()
	j.cancel = cancel
	j.mu.Unlock()
	defer cancel()

	err := m.encoder.Encode(jobCtx, j.SourcePath, j.OutputPath, j.Params, j.SourceInfo.DurationS, j.setProgress)

	switch {
	case err == nil:
		j.complete(m.outputRetention)
	case jobCtx.Err() == context.DeadlineExceeded:
		// REQ-JOB-9: exceeded configured maximum duration.
		os.Remove(j.OutputPath)
		j.fail(StateTimedOut, "encoding exceeded the maximum allowed duration")
	case jobCtx.Err() == context.Canceled:
		// REQ-JOB-4: user-initiated cancellation released resources.
		os.Remove(j.OutputPath)
		j.fail(StateCancelled, "cancelled by user")
	default:
		// REQ-JOB-7/8: engine failure or crash.
		os.Remove(j.OutputPath)
		j.fail(StateFailed, err.Error())
	}
}

// retentionSweeper deletes expired outputs and marks their jobs expired
// (REQ-OUT-3), checking periodically until ctx is cancelled.
func (m *Manager) retentionSweeper(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			m.sweepExpired(now)
		}
	}
}

func (m *Manager) sweepExpired(now time.Time) {
	m.mu.RLock()
	jobs := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	m.mu.RUnlock()

	for _, j := range jobs {
		if !j.IsExpired(now) {
			continue
		}
		os.Remove(j.OutputPath)
		j.mu.Lock()
		j.state = StateExpired
		j.mu.Unlock()
	}
}

// NewID generates a unique job identifier. Exposed so callers can compute
// derived resources (like an output file path) before calling Submit.
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
