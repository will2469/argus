package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"
)


// NewRequestTracker creates a new thread-safe RequestTracker.
func NewRequestTracker() *RequestTracker {
	return &RequestTracker{
		inFlight: make(map[string]inFlightEntry),
	}
}

// IDKey computes a collision-free, canonical string representation of a JSON-RPC request ID.
// In JSON-RPC 2.0, an ID may be a string, number, or null. Using fmt.Sprintf("%v", id)
// causes collisions between string "1" and number 1. IDKey disambiguates by prepending
// disjoint type prefixes: "s:" for strings and "n:" for numbers.
func IDKey(id any) string {
	switch v := id.(type) {
	case string:
		return "s:" + v
	case float64:
		if math.Floor(v) == v && !math.IsInf(v, 0) && !math.IsNaN(v) && v >= math.MinInt64 && v <= math.MaxInt64 {
			return "n:" + strconv.FormatInt(int64(v), 10)
		}
		return "n:" + strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		f64 := float64(v)
		if math.Floor(f64) == f64 && !math.IsInf(f64, 0) && !math.IsNaN(f64) && f64 >= math.MinInt64 && f64 <= math.MaxInt64 {
			return "n:" + strconv.FormatInt(int64(f64), 10)
		}
		return "n:" + strconv.FormatFloat(f64, 'f', -1, 64)
	case int:
		return "n:" + strconv.FormatInt(int64(v), 10)
	case int64:
		return "n:" + strconv.FormatInt(v, 10)
	case int32:
		return "n:" + strconv.FormatInt(int64(v), 10)
	case int16:
		return "n:" + strconv.FormatInt(int64(v), 10)
	case int8:
		return "n:" + strconv.FormatInt(int64(v), 10)
	case uint:
		return "n:" + strconv.FormatUint(uint64(v), 10)
	case uint64:
		return "n:" + strconv.FormatUint(v, 10)
	case uint32:
		return "n:" + strconv.FormatUint(uint64(v), 10)
	case uint16:
		return "n:" + strconv.FormatUint(uint64(v), 10)
	case uint8:
		return "n:" + strconv.FormatUint(uint64(v), 10)
	case json.Number:
		return "n:" + string(v)
	case nil:
		return ""
	default:
		return fmt.Sprintf("u:%T:%v", id, id)
	}
}

func idKey(id any) string {
	return IDKey(id)
}

// Begin registers an in-flight request with a monotonic generation token and returns a cancellable context
// along with a completion function that safely unregisters only this generation upon completion.
func (rt *RequestTracker) Begin(parent context.Context, id any) (context.Context, context.CancelFunc) {
	if id == nil {
		return parent, func() {}
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.nextGen++
	gen := rt.nextGen
	ctx, cancel := context.WithCancel(parent)
	rt.inFlight[idKey(id)] = inFlightEntry{
		generation: gen,
		cancel:     cancel,
	}

	var once sync.Once
	endFunc := func() {
		once.Do(func() {
			cancel()
			rt.End(id, gen)
		})
	}
	return ctx, endFunc
}

// End unregisters an in-flight request when execution finishes.
// If token is provided, it guards against ABA races: it only deletes the map entry
// if its generation matches the specified token.
func (rt *RequestTracker) End(id any, token ...uint64) {
	if id == nil {
		return
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	key := idKey(id)
	entry, exists := rt.inFlight[key]
	if !exists {
		return
	}
	if len(token) > 0 && token[0] != 0 && entry.generation != token[0] {
		return // ABA guard: entry was replaced by a newer generation!
	}
	delete(rt.inFlight, key)
}

// Cancel triggers cancellation for a specific in-flight requestId.
// It cancels the context and marks the entry as cancelled, but DOES NOT delete the entry
// immediately to prevent ABA races when IDs are reused before the in-flight goroutine completes.
// Returns true if an active uncancelled request with that ID was found and cancelled.
func (rt *RequestTracker) Cancel(requestId any) bool {
	if requestId == nil {
		return false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()

	key := idKey(requestId)
	entry, exists := rt.inFlight[key]
	if !exists || entry.cancelled {
		return false
	}
	entry.cancelled = true
	rt.inFlight[key] = entry
	entry.cancel()
	return true
}

// ActiveCount returns the number of currently active, non-cancelled tracked requests.
func (rt *RequestTracker) ActiveCount() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	count := 0
	for _, entry := range rt.inFlight {
		if !entry.cancelled {
			count++
		}
	}
	return count
}
