package job

import (
	"context"
	"sync"
)

// Queue is the interface workers pull job IDs from. It is intentionally
// minimal (FIFO push/pop) so the in-memory v1 implementation can later be
// swapped for a Redis- or NATS-backed queue, per Kickoff.md §3.
type Queue interface {
	// Push enqueues a job ID and returns its 0-based position from the back
	// of the queue at the moment of insertion.
	Push(id string) (position int)
	// Pop blocks until a job ID is available or ctx is cancelled.
	Pop(ctx context.Context) (id string, ok bool)
	// Len returns the current number of queued (not yet popped) IDs.
	Len() int
}

// memQueue is an unbounded, mutex-guarded FIFO. v1 accepts jobs without a
// hard capacity rejection: when the queue is long, jobs are simply placed in
// pending state with their queue position reported (REQ-JOB-6) rather than
// turned away.
type memQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []string
	closed bool
}

// NewMemoryQueue constructs the v1 in-process Queue implementation.
func NewMemoryQueue() Queue {
	q := &memQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *memQueue) Push(id string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	pos := len(q.items)
	q.items = append(q.items, id)
	q.cond.Signal()
	return pos
}

func (q *memQueue) Pop(ctx context.Context) (string, bool) {
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			q.mu.Lock()
			q.cond.Broadcast()
			q.mu.Unlock()
		case <-done:
		}
	}()

	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		if ctx.Err() != nil {
			return "", false
		}
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return "", false
	}
	id := q.items[0]
	q.items = q.items[1:]
	return id, true
}

func (q *memQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
