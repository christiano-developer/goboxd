// internal/worker/pool_test.go
package worker

import (
	"context"
	"testing"
	"time"
)

func TestRateTracker(t *testing.T) {
	rt := NewRateTracker(1 * time.Second)
	rt.AddRequest()
	rt.AddRequest()

	rate := rt.GetRate()
	if rate != 2.0 {
		t.Errorf("expected rate of 2.0, got %f", rate)
	}

	time.Sleep(1100 * time.Millisecond)
	rate = rt.GetRate()
	if rate != 0.0 {
		t.Errorf("expected rate of 0.0 after expiration, got %f", rate)
	}
}

func TestPoolPrioritySortingAndAging(t *testing.T) {
	// 1 active, 5 queue size
	cp := NewConcurrencyPool(1, 5)

	// Block the pool with first job
	ctx1 := context.Background()
	err := cp.Acquire(ctx1, 10, 100)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Enqueue a heavy job (cost=100, memory=500000)
	ctx2 := context.Background()
	ch2 := make(chan error, 1)
	go func() {
		ch2 <- cp.Acquire(ctx2, 100, 500000)
	}()

	// Enqueue a light job (cost=1, memory=100)
	time.Sleep(50 * time.Millisecond) // Ensure clear enqueue order
	ctx3 := context.Background()
	ch3 := make(chan error, 1)
	go func() {
		ch3 <- cp.Acquire(ctx3, 1, 100)
	}()

	// Release first job. The light job (ch3) has lower cost and should run first!
	time.Sleep(50 * time.Millisecond)
	cp.Release()

	select {
	case err := <-ch3:
		if err != nil {
			t.Errorf("ch3 failed: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for light job")
	}

	// Release second job. Now the heavy job (ch2) runs
	cp.Release()

	select {
	case err := <-ch2:
		if err != nil {
			t.Errorf("ch2 failed: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for heavy job")
	}
}

func TestPoolQueueLimitsAndShedding(t *testing.T) {
	// 1 active, 1 queue size
	cp := NewConcurrencyPool(1, 1)

	// Acquire active slot
	err := cp.Acquire(context.Background(), 10, 100)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	// Enqueue one job (fills the queue of size 1)
	ch2 := make(chan error, 1)
	go func() {
		ch2 <- cp.Acquire(context.Background(), 10, 100)
	}()

	time.Sleep(50 * time.Millisecond)

	// Try to enqueue a third job -> should immediately return ErrQueueFull
	err = cp.Acquire(context.Background(), 10, 100)
	if err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}

	// Release first, letting second job run
	cp.Release()

	select {
	case err := <-ch2:
		if err != nil {
			t.Errorf("second job failed: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for second job")
	}
}
