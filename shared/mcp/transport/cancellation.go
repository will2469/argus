package transport

import (
	"context"
	"fmt"
	"sync"
)

// ErrorCodeCancelled is the standard JSON-RPC/MCP error code for cancelled requests.
const ErrorCodeCancelled = -32800

// inFlightEntry holds the cancellation trigger for an active request.
type inFlightEntry struct {
	cancel context.CancelFunc
}

// RequestTracker manages per-request cancellation contexts in a thread-safe manner.
type RequestTracker struct {
	mu       sync.Mutex
	inFlight map[string]inFlightEntry
}

// NewRequestTracker creates a new thread-safe RequestTracker.
func NewRequestTracker() *RequestTracker {
	return &RequestTracker{
		inFlight: make(map[string]inFlightEntry),
	}
}

func idKey(id any) string {
	return fmt.Sprintf("%v", id)
}

// Begin registers an in-flight request and returns a cancellable context.
func (rt *RequestTracker) Begin(parent context.Context, id any) (context.Context, context.CancelFunc) {
	if id == nil {
		return parent, func() {}
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	ctx, cancel := context.WithCancel(parent)
	rt.inFlight[idKey(id)] = inFlightEntry{cancel: cancel}
	return ctx, cancel
}

// End unregisters an in-flight request when execution finishes.
func (rt *RequestTracker) End(id any) {
	if id == nil {
		return
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	delete(rt.inFlight, idKey(id))
}

// Cancel triggers cancellation for a specific in-flight requestId.
// Returns true if a running request with that ID was found and cancelled.
func (rt *RequestTracker) Cancel(requestId any) bool {
	if requestId == nil {
		return false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	key := idKey(requestId)
	entry, exists := rt.inFlight[key]
	if !exists {
		return false
	}
	entry.cancel()
	delete(rt.inFlight, key)
	return true
}

// ActiveCount returns the number of currently tracked in-flight requests.
func (rt *RequestTracker) ActiveCount() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return len(rt.inFlight)
}
