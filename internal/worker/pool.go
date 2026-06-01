// internal/worker/pool.go
package worker

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"time"
)

var ErrQueueFull = errors.New("queue is full")

// QueueItem represents a pending request waiting in the priority queue.
type QueueItem struct {
	Ctx         context.Context
	Cost        int64         // Wall Time S * Memory multiplier
	MemoryKB    int           // Memory requested
	EnqueueTime time.Time
	Ch          chan struct{} // Closed when this item is popped to run
	index       int           // Required for container/heap
}

// PriorityQueue implements heap.Interface and holds QueueItems.
type PriorityQueue []*QueueItem

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	now := time.Now()
	score := func(item *QueueItem) float64 {
		// Base score is expected duration in seconds + memory weight
		base := float64(item.Cost) * (1.0 + float64(item.MemoryKB)/1048576.0)
		// Wait time in seconds
		waitSecs := now.Sub(item.EnqueueTime).Seconds()
		// Starvation prevention: subtract weighted wait time (2.0 points/sec)
		return base - 2.0*waitSecs
	}
	return score(pq[i]) < score(pq[j])
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*QueueItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

// ConcurrencyPool manages concurrent job scheduling, queueing, and load shedding.
type ConcurrencyPool struct {
	mu          sync.Mutex
	pq          PriorityQueue
	maxActive   int
	maxQueue    int
	activeCount int
	shedCount   uint64

	RateTracker      *RateTracker
	ExecutionTracker *ExecutionTracker
}

func NewConcurrencyPool(maxActive, maxQueue int) *ConcurrencyPool {
	cp := &ConcurrencyPool{
		maxActive:        maxActive,
		maxQueue:         maxQueue,
		RateTracker:      NewRateTracker(10 * time.Second),
		ExecutionTracker: NewExecutionTracker(100),
	}
	heap.Init(&cp.pq)
	return cp
}

// Acquire blocks until an execution slot is free, respecting context cancellation.
func (cp *ConcurrencyPool) Acquire(ctx context.Context, cost int64, memKB int) error {
	cp.RateTracker.AddRequest()

	cp.mu.Lock()
	// Shed load if active + queue exceeds pool limit
	if cp.activeCount+cp.pq.Len() >= cp.maxActive+cp.maxQueue {
		cp.shedCount++
		cp.mu.Unlock()
		return ErrQueueFull
	}

	// Dispatch immediately if slots are open and nobody is queued
	if cp.activeCount < cp.maxActive && cp.pq.Len() == 0 {
		cp.activeCount++
		cp.mu.Unlock()
		return nil
	}

	// Enqueue
	ch := make(chan struct{})
	item := &QueueItem{
		Ctx:         ctx,
		Cost:        cost,
		MemoryKB:    memKB,
		EnqueueTime: time.Now(),
		Ch:          ch,
	}
	heap.Push(&cp.pq, item)
	cp.mu.Unlock()

	// Wait for dispatch or client context cancellation
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		cp.mu.Lock()
		if item.index >= 0 {
			heap.Remove(&cp.pq, item.index)
		}
		cp.mu.Unlock()
		return ctx.Err()
	}
}

// Release frees an execution slot and schedules the next job in queue.
func (cp *ConcurrencyPool) Release() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.activeCount--
	cp.scheduleNext()
}

func (cp *ConcurrencyPool) scheduleNext() {
	// Re-initialize the heap to force re-sorting based on updated aging wait times
	if cp.pq.Len() > 0 {
		heap.Init(&cp.pq)
	}

	for cp.activeCount < cp.maxActive && cp.pq.Len() > 0 {
		item := heap.Pop(&cp.pq).(*QueueItem)
		if item.Ctx.Err() != nil {
			continue // Already cancelled, skip
		}
		cp.activeCount++
		close(item.Ch)
	}
}

func (cp *ConcurrencyPool) GetStats() (active, queued int, shed uint64) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	return cp.activeCount, cp.pq.Len(), cp.shedCount
}

func (cp *ConcurrencyPool) GetMaxActive() int {
	return cp.maxActive
}

func (cp *ConcurrencyPool) GetMaxQueue() int {
	return cp.maxQueue
}

// RateTracker computes the request rate (req/sec) over a sliding window.
type RateTracker struct {
	mu       sync.Mutex
	requests []time.Time
	window   time.Duration
}

func NewRateTracker(window time.Duration) *RateTracker {
	return &RateTracker{
		window: window,
	}
}

func (rt *RateTracker) AddRequest() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.requests = append(rt.requests, time.Now())
	rt.cleanup(time.Now())
}

func (rt *RateTracker) GetRate() float64 {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	now := time.Now()
	rt.cleanup(now)
	return float64(len(rt.requests)) / rt.window.Seconds()
}

func (rt *RateTracker) cleanup(now time.Time) {
	cutoff := now.Add(-rt.window)
	idx := 0
	for i, t := range rt.requests {
		if t.After(cutoff) {
			idx = i
			break
		}
		if i == len(rt.requests)-1 {
			idx = len(rt.requests)
		}
	}
	rt.requests = rt.requests[idx:]
}

// ExecutionTracker calculates a moving average duration of successful executions.
type ExecutionTracker struct {
	mu        sync.RWMutex
	durations []time.Duration
	maxSize   int
}

func NewExecutionTracker(maxSize int) *ExecutionTracker {
	return &ExecutionTracker{
		maxSize: maxSize,
	}
}

func (et *ExecutionTracker) AddDuration(d time.Duration) {
	et.mu.Lock()
	defer et.mu.Unlock()
	et.durations = append(et.durations, d)
	if len(et.durations) > et.maxSize {
		et.durations = et.durations[1:]
	}
}

func (et *ExecutionTracker) GetAverage() time.Duration {
	et.mu.RLock()
	defer et.mu.RUnlock()
	if len(et.durations) == 0 {
		return 300 * time.Millisecond // Default fallback duration
	}
	var total time.Duration
	for _, d := range et.durations {
		total += d
	}
	return total / time.Duration(len(et.durations))
}
