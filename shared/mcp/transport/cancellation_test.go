package transport

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRequestTracker_Lifecycle(t *testing.T) {
	tracker := NewRequestTracker()

	// 1. Begin request tracking
	ctx, cancel := tracker.Begin(context.Background(), "req-1")
	defer cancel()

	if tracker.ActiveCount() != 1 {
		t.Fatalf("expected 1 active request, got %d", tracker.ActiveCount())
	}

	// 2. Context should still be active
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done yet")
	default:
	}

	// 3. Cancel request via tracker
	cancelled := tracker.Cancel("req-1")
	if !cancelled {
		t.Fatal("expected Cancel to return true for active request")
	}

	// 4. Context should now be cancelled
	select {
	case <-ctx.Done():
		// Success
	case <-time.After(50 * time.Millisecond):
		t.Fatal("context was not cancelled after tracker.Cancel()")
	}

	// 5. Active count should decrement
	if tracker.ActiveCount() != 0 {
		t.Fatalf("expected 0 active requests, got %d", tracker.ActiveCount())
	}

	// 6. Cancel again should return false (already cancelled/removed)
	if tracker.Cancel("req-1") {
		t.Fatal("expected second Cancel to return false")
	}
}

func TestRequestTracker_Concurrency(t *testing.T) {
	tracker := NewRequestTracker()
	const numGoroutines = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			ctx, cancel := tracker.Begin(context.Background(), id)
			defer cancel()

			time.Sleep(5 * time.Millisecond)
			tracker.End(id)
			_ = ctx
		}(i)
	}

	wg.Wait()

	if tracker.ActiveCount() != 0 {
		t.Fatalf("expected all requests to be cleaned up, got %d active", tracker.ActiveCount())
	}
}
