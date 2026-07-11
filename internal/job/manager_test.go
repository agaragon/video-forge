package job

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/agaragon/video-forge/internal/media"
)

// fakeEncoder lets tests control encode duration/outcome without shelling
// out to ffmpeg.
type fakeEncoder struct {
	mu       sync.Mutex
	delay    time.Duration
	fail     error
	progress []media.Progress
}

func (f *fakeEncoder) Encode(ctx context.Context, inputPath, outputPath string, p media.Params, totalDurationS float64, onProgress func(media.Progress)) error {
	onProgress(media.Progress{Percent: 0})
	select {
	case <-time.After(f.delay):
	case <-ctx.Done():
		return ctx.Err()
	}
	f.mu.Lock()
	fail := f.fail
	f.mu.Unlock()
	if fail != nil {
		return fail
	}
	onProgress(media.Progress{Percent: 100, Done: true})
	return nil
}

func waitForState(t *testing.T, j *Job, want State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if j.State() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s: state = %q, want %q", j.ID, j.State(), want)
}

func TestManagerSubmitAndComplete(t *testing.T) {
	enc := &fakeEncoder{delay: 10 * time.Millisecond}
	m := NewManager(enc, 1, time.Second, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	j := m.Submit(NewID(), media.KindVideo, "in.mp4", "out.mp4", media.Info{}, media.Params{})
	if j.State() != StatePending {
		t.Fatalf("initial state = %q, want pending", j.State())
	}

	waitForState(t, j, StateCompleted, time.Second)
	snap := j.Snapshot()
	if snap.Progress.Percent != 100 {
		t.Errorf("final progress = %v, want 100", snap.Progress.Percent)
	}
	if snap.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if snap.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set on completion")
	}
}

func TestManagerJobFailure(t *testing.T) {
	enc := &fakeEncoder{delay: time.Millisecond, fail: errors.New("boom")}
	m := NewManager(enc, 1, time.Second, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	j := m.Submit(NewID(), media.KindVideo, "in.mp4", "out.mp4", media.Info{}, media.Params{})
	waitForState(t, j, StateFailed, time.Second)
	if j.Snapshot().Error == "" {
		t.Error("expected failure reason to be recorded")
	}
}

func TestManagerCancelRunningJob(t *testing.T) {
	enc := &fakeEncoder{delay: 5 * time.Second}
	m := NewManager(enc, 1, time.Minute, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	j := m.Submit(NewID(), media.KindVideo, "in.mp4", "out.mp4", media.Info{}, media.Params{})
	waitForState(t, j, StateRunning, time.Second)

	if err := m.Cancel(j.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	waitForState(t, j, StateCancelled, time.Second)
}

func TestManagerCancelPendingJob(t *testing.T) {
	enc := &fakeEncoder{delay: 200 * time.Millisecond}
	m := NewManager(enc, 1, time.Minute, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	// Occupy the single worker so the second job stays pending.
	busy := m.Submit(NewID(), media.KindVideo, "busy.mp4", "busy-out.mp4", media.Info{}, media.Params{})
	waitForState(t, busy, StateRunning, time.Second)

	pending := m.Submit(NewID(), media.KindVideo, "in.mp4", "out.mp4", media.Info{}, media.Params{})
	if pending.State() != StatePending {
		t.Fatalf("state = %q, want pending", pending.State())
	}

	if err := m.Cancel(pending.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if pending.State() != StateCancelled {
		t.Fatalf("state = %q, want cancelled", pending.State())
	}
}

func TestManagerJobTimeout(t *testing.T) {
	enc := &fakeEncoder{delay: 5 * time.Second}
	m := NewManager(enc, 1, 50*time.Millisecond, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	j := m.Submit(NewID(), media.KindVideo, "in.mp4", "out.mp4", media.Info{}, media.Params{})
	waitForState(t, j, StateTimedOut, time.Second)
}

func TestManagerCancelUnknownJob(t *testing.T) {
	m := NewManager(&fakeEncoder{}, 1, time.Second, time.Hour)
	if err := m.Cancel("does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestManagerQueuePosition(t *testing.T) {
	enc := &fakeEncoder{delay: 200 * time.Millisecond}
	m := NewManager(enc, 1, time.Minute, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	first := m.Submit(NewID(), media.KindVideo, "a.mp4", "a-out.mp4", media.Info{}, media.Params{})
	waitForState(t, first, StateRunning, time.Second)

	second := m.Submit(NewID(), media.KindVideo, "b.mp4", "b-out.mp4", media.Info{}, media.Params{})
	if second.Snapshot().QueuePosition != 0 {
		t.Errorf("QueuePosition = %d, want 0 (next in line)", second.Snapshot().QueuePosition)
	}
}
