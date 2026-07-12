// Package job implements the async job queue and worker pool described in
// Kickoff.md §3 ("in-process queue for v1; pluggable interface so it can be
// swapped for Redis/NATS later") and the REQ-JOB-* / REQ-OUT-* requirements.
package job

import (
	"context"
	"sync"
	"time"

	"github.com/agaragon/video-forge/internal/media"
)

// State is the lifecycle state of a Job (REQ-JOB-*, REQ-OUT-3).
type State string

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateCompleted State = "completed"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
	StateTimedOut  State = "timed_out"
	StateExpired   State = "expired"
)

// Job is one encode request moving through the queue and worker pool. All
// mutable fields are guarded by mu so API handlers and workers can access a
// Job concurrently.
type Job struct {
	ID         string
	Kind       media.Kind
	SourcePath string
	SourceInfo media.Info
	Params     media.Params
	OutputPath string
	CreatedAt  time.Time

	mu            sync.Mutex
	state         State
	progress      media.Progress
	queuePosition int
	errMsg        string
	startedAt     time.Time
	completedAt   time.Time
	expiresAt     time.Time
	cancel        context.CancelFunc
}

// View is the JSON-serializable snapshot of a Job returned by the API.
type View struct {
	ID            string         `json:"id"`
	Kind          media.Kind     `json:"kind"`
	State         State          `json:"state"`
	Progress      media.Progress `json:"progress"`
	QueuePosition int            `json:"queue_position,omitempty"`
	Error         string         `json:"error,omitempty"`
	SourceInfo    media.Info     `json:"source_info"`
	Params        media.Params   `json:"params"`
	CreatedAt     time.Time      `json:"created_at"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	ExpiresAt     *time.Time     `json:"expires_at,omitempty"`
}

func newJob(id string, kind media.Kind, sourcePath, outputPath string, info media.Info, params media.Params) *Job {
	return &Job{
		ID:         id,
		Kind:       kind,
		SourcePath: sourcePath,
		OutputPath: outputPath,
		SourceInfo: info,
		Params:     params,
		CreatedAt:  time.Now().UTC(),
		state:      StatePending,
	}
}

// State returns the job's current lifecycle state.
func (j *Job) State() State {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.state
}

func (j *Job) setState(s State) {
	j.mu.Lock()
	j.state = s
	j.mu.Unlock()
}

// setProgress records the latest checkpoint reported by the encoder (REQ-JOB-2).
func (j *Job) setProgress(p media.Progress) {
	j.mu.Lock()
	j.progress = p
	j.mu.Unlock()
}

func (j *Job) setQueuePosition(pos int) {
	j.mu.Lock()
	j.queuePosition = pos
	j.mu.Unlock()
}

// fail records a terminal failure with its reason (REQ-JOB-7).
func (j *Job) fail(state State, reason string) {
	j.mu.Lock()
	j.state = state
	j.errMsg = reason
	now := time.Now().UTC()
	j.completedAt = now
	j.mu.Unlock()
}

func (j *Job) complete(retention time.Duration) {
	j.mu.Lock()
	j.state = StateCompleted
	now := time.Now().UTC()
	j.completedAt = now
	exp := now.Add(retention)
	j.expiresAt = exp
	j.mu.Unlock()
}

// Snapshot returns a point-in-time, JSON-safe copy of the job's state.
func (j *Job) Snapshot() View {
	j.mu.Lock()
	defer j.mu.Unlock()

	v := View{
		ID:            j.ID,
		Kind:          j.Kind,
		State:         j.state,
		Progress:      j.progress,
		QueuePosition: j.queuePosition,
		Error:         j.errMsg,
		SourceInfo:    j.SourceInfo,
		Params:        j.Params,
		CreatedAt:     j.CreatedAt,
	}
	if !j.startedAt.IsZero() {
		t := j.startedAt
		v.StartedAt = &t
	}
	if !j.completedAt.IsZero() {
		t := j.completedAt
		v.CompletedAt = &t
	}
	if !j.expiresAt.IsZero() {
		t := j.expiresAt
		v.ExpiresAt = &t
	}
	return v
}

// IsExpired reports whether a completed job's output has passed its
// retention deadline (REQ-OUT-3).
func (j *Job) IsExpired(now time.Time) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.state == StateCompleted && !j.expiresAt.IsZero() && now.After(j.expiresAt)
}
