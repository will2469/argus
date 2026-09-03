package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)

const (
	// DefaultMaxConcurrentCheap limits parallel lightweight requests (ping, rule lookup, tools/list).
	DefaultMaxConcurrentCheap = 32
	// DefaultMaxConcurrentExpensive limits heavy static analysis / migration parsers.
	DefaultMaxConcurrentExpensive = 2
	// DefaultShutdownTimeout is the maximum duration to wait for in-flight requests during server shutdown.
	DefaultShutdownTimeout = 5 * time.Second
)

// ResourceCost classifies the concurrency cost of requests.
type ResourceCost string

const (
	// CostCheap designates non-blocking, memory-light operations (ping, inspect rule, list tools).
	CostCheap ResourceCost = "cheap"
	// CostExpensive designates CPU and I/O intensive operations (full repo scan, migration parse).
	CostExpensive ResourceCost = "expensive"
)

// HandlerFunc executes a validated JSON-RPC request and returns a response.
type HandlerFunc func(context.Context, ParsedRequest) *mcperrors.JSONRPCResponse

// SynchronizedWriter ensures JSON-RPC responses are written sequentially to stdout without interleaving.
type SynchronizedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewSynchronizedWriter creates a thread-safe JSON-RPC response writer.
func NewSynchronizedWriter(w io.Writer) *SynchronizedWriter {
	return &SynchronizedWriter{w: w}
}

// WriteResponse serializes and writes a JSON-RPC response as a single newline-delimited JSON line.
// If serialization fails, it attempts to write a fallback JSON-RPC internal error (-32603) so the
// client's pending call is not left hanging, while returning the marshal error to the caller.
func (sw *SynchronizedWriter) WriteResponse(resp mcperrors.JSONRPCResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		fallback := mcperrors.ProtocolError(resp.ID, mcperrors.CodeInternalError, fmt.Sprintf("Response serialization failed: %v", err))
		if fbData, fbErr := json.Marshal(fallback); fbErr == nil {
			sw.mu.Lock()
			_, _ = fmt.Fprintf(sw.w, "%s\n", fbData)
			sw.mu.Unlock()
		}
		return fmt.Errorf("failed to marshal jsonrpc response: %w", err)
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()
	_, err = fmt.Fprintf(sw.w, "%s\n", data)
	return err
}

// Dispatcher coordinates request concurrency, lifecycle isolation, cancellation, and graceful shutdown.
// It partitions resources into Cheap and Expensive semaphore pools to prevent Head-of-Line blocking.
type Dispatcher struct {
	tracker      *RequestTracker
	writer       *SynchronizedWriter
	cheapSem     chan struct{}
	expensiveSem chan struct{}
	wg           sync.WaitGroup
	rootCtx      context.Context
	cancelAll    context.CancelFunc
	errMu        sync.Mutex
	lastErr      error
}

// NewDispatcher initializes a Dispatcher with dual semaphore pools and synchronized output.
func NewDispatcher(w io.Writer, maxExpensive int, maxCheap int) *Dispatcher {
	if maxExpensive <= 0 {
		maxExpensive = DefaultMaxConcurrentExpensive
	}
	if maxCheap <= 0 {
		maxCheap = DefaultMaxConcurrentCheap
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Dispatcher{
		tracker:      NewRequestTracker(),
		writer:       NewSynchronizedWriter(w),
		cheapSem:     make(chan struct{}, maxCheap),
		expensiveSem: make(chan struct{}, maxExpensive),
		rootCtx:      ctx,
		cancelAll:    cancel,
	}
}

func (d *Dispatcher) recordError(err error) {
	if err == nil {
		return
	}
	d.errMu.Lock()
	defer d.errMu.Unlock()
	if d.lastErr == nil {
		d.lastErr = err
	}
}

// Err returns the first fatal write or serialization error encountered, if any.
func (d *Dispatcher) Err() error {
	d.errMu.Lock()
	defer d.errMu.Unlock()
	return d.lastErr
}

// Dispatch dispatches a validated JSON-RPC request into an isolated, managed goroutine
// with partitioned semaphore concurrency and cancellation tracking.
func (d *Dispatcher) Dispatch(req ParsedRequest, cost ResourceCost, handler HandlerFunc) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		// Select the appropriate pool: heavy scans never exhaust cheap slots
		var sem chan struct{}
		if cost == CostExpensive {
			sem = d.expensiveSem
		} else {
			sem = d.cheapSem
		}

		ctx, cancel := d.tracker.Begin(d.rootCtx, req.ID)
		defer func() {
			cancel()
			d.tracker.End(req.ID)
		}()

		// Acquire concurrency semaphore slot. If cancelled while queued, abort early!
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		case <-d.rootCtx.Done():
			return
		case <-ctx.Done():
			resp := mcperrors.CancelledError(req.ID, "Request cancelled while queued")
			if err := d.writer.WriteResponse(*resp); err != nil {
				fmt.Fprintf(os.Stderr, "mcp: failed to write cancelled response for %v: %v\n", req.ID, err)
				d.recordError(err)
			}
			return
		}

		resp := handler(ctx, req)
		if resp != nil {
			if err := d.writer.WriteResponse(*resp); err != nil {
				fmt.Fprintf(os.Stderr, "mcp: failed to write response for %v: %v\n", req.ID, err)
				d.recordError(err)
			}
		}
	}()
}

// HandleCancel triggers cancellation on an in-flight request ID.
func (d *Dispatcher) HandleCancel(requestID any) bool {
	return d.tracker.Cancel(requestID)
}

// WriteResponse writes a response directly through the synchronized writer.
func (d *Dispatcher) WriteResponse(resp mcperrors.JSONRPCResponse) error {
	err := d.writer.WriteResponse(resp)
	if err != nil {
		d.recordError(err)
	}
	return err
}

// ActiveRequests returns the number of active tracked requests.
func (d *Dispatcher) ActiveRequests() int {
	return d.tracker.ActiveCount()
}

// Shutdown gracefully waits for active requests to finish or cancels them if timeout expires.
func (d *Dispatcher) Shutdown(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Clean exit: all requests completed within timeout
	case <-time.After(timeout):
		// Timeout exceeded: abort remaining requests
		d.cancelAll()
		<-done
	}
}
