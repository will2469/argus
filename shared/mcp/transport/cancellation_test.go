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

func TestRequestTracker_IDCollisionAdversarial(t *testing.T) {
	// 1. Verify canonical ID keys are strictly disjoint across types
	if IDKey("1") != "s:1" {
		t.Fatalf("expected s:1, got %s", IDKey("1"))
	}
	if IDKey(1) != "n:1" {
		t.Fatalf("expected n:1, got %s", IDKey(1))
	}
	if IDKey(float64(1.0)) != "n:1" {
		t.Fatalf("expected n:1 for float64(1.0), got %s", IDKey(float64(1.0)))
	}
	if IDKey(int64(1)) != "n:1" {
		t.Fatalf("expected n:1 for int64(1), got %s", IDKey(int64(1)))
	}
	if IDKey("n:1") != "s:n:1" {
		t.Fatalf("expected s:n:1 for string 'n:1', got %s", IDKey("n:1"))
	}
	if IDKey(nil) != "" {
		t.Fatalf("expected empty key for nil, got %s", IDKey(nil))
	}

	tracker := NewRequestTracker()

	// 2. Register Request A (integer 1) and Request B (string "1") simultaneously
	ctxA, cancelA := tracker.Begin(context.Background(), 1)
	defer cancelA()

	ctxB, cancelB := tracker.Begin(context.Background(), "1")
	defer cancelB()

	// Both requests MUST coexist without overwriting each other
	if tracker.ActiveCount() != 2 {
		t.Fatalf("expected 2 active requests, got %d (overwrite collision detected!)", tracker.ActiveCount())
	}

	// 3. Cancel Request A (number 1)
	cancelledA := tracker.Cancel(1)
	if !cancelledA {
		t.Fatal("expected Cancel(1) to succeed")
	}

	// Request A must be cancelled
	select {
	case <-ctxA.Done():
		// Success
	default:
		t.Fatal("ctxA should be cancelled")
	}

	// Request B ("1") MUST NOT be cancelled (no cross-type cancellation leakage)
	select {
	case <-ctxB.Done():
		t.Fatal("ctxB was erroneously cancelled when cancelling numeric ID 1!")
	default:
		// Success: Request B is still alive
	}

	if tracker.ActiveCount() != 1 {
		t.Fatalf("expected 1 remaining active request, got %d", tracker.ActiveCount())
	}

	// 4. End Request B cleanly
	tracker.End("1")
	if tracker.ActiveCount() != 0 {
		t.Fatalf("expected 0 active requests after End('1'), got %d", tracker.ActiveCount())
	}

	// 5. Cross-type numeric cancellation: int(42) registered, float64(42.0) cancelled (from JSON unmarshaling)
	ctxC, cancelC := tracker.Begin(context.Background(), int(42))
	defer cancelC()

	if !tracker.Cancel(float64(42.0)) {
		t.Fatal("expected Cancel(float64(42.0)) to cancel registered int(42)")
	}
	select {
	case <-ctxC.Done():
		// Success
	default:
		t.Fatal("ctxC should be cancelled via float64(42.0)")
	}

	// 6. Tricky namespace collision test: string "n:1" vs number 1
	ctxNum, cancelNum := tracker.Begin(context.Background(), 1)
	defer cancelNum()
	ctxStrPrefixed, cancelStrPrefixed := tracker.Begin(context.Background(), "n:1")
	defer cancelStrPrefixed()

	if tracker.ActiveCount() != 2 {
		t.Fatalf("expected 2 active requests for 1 and 'n:1', got %d", tracker.ActiveCount())
	}

	if !tracker.Cancel("n:1") {
		t.Fatal("expected Cancel('n:1') to succeed")
	}
	select {
	case <-ctxStrPrefixed.Done():
		// Success
	default:
		t.Fatal("ctxStrPrefixed should be cancelled")
	}
	select {
	case <-ctxNum.Done():
		t.Fatal("ctxNum was erroneously cancelled when cancelling string 'n:1'!")
	default:
		// Success: numeric 1 still alive
	}
	tracker.End(1)
}

// TestRequestTracker_ABARaceProtection proves that reusing an ID while an earlier cancelled
// request is still unwinding does not allow the earlier request's completion to wipe out
// the newer generation's tracker registration.
func TestRequestTracker_ABARaceProtection(t *testing.T) {
	tracker := NewRequestTracker()

	// 1. Request A begins with id=1 (Generation 1)
	ctxA, endA := tracker.Begin(context.Background(), 1)

	// 2. Request A is cancelled
	if !tracker.Cancel(1) {
		t.Fatal("expected Cancel(1) to succeed for request A")
	}
	select {
	case <-ctxA.Done():
		// Success
	default:
		t.Fatal("ctxA should be cancelled")
	}

	// 3. Before Request A unwinds and calls endA(), Request B reuses id=1 (Generation 2)
	ctxB, endB := tracker.Begin(context.Background(), 1)
	defer endB()

	if tracker.ActiveCount() != 1 {
		t.Fatalf("expected 1 active request for generation 2, got %d", tracker.ActiveCount())
	}

	// 4. Request A finally finishes and executes its deferred endA()
	endA()

	// ABA Guard Assertion: Request B MUST STILL BE TRACKED and active!
	if tracker.ActiveCount() != 1 {
		t.Fatalf("ABA race violation: Request A's End() destroyed Request B's registration! ActiveCount=%d", tracker.ActiveCount())
	}

	// 5. Request B can still be cancelled successfully
	if !tracker.Cancel(1) {
		t.Fatal("expected Cancel(1) to succeed for Request B")
	}
	select {
	case <-ctxB.Done():
		// Success: Request B received cancellation
	default:
		t.Fatal("ctxB should be cancelled")
	}
}
