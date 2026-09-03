package transport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)

// ResourceCost classifies the concurrency cost of requests.
type ResourceCost string

const (
	// CostCheap designates non-blocking, memory-light operations (ping, inspect rule, list tools).
	CostCheap ResourceCost = "cheap"
	// CostExpensive designates CPU and I/O intensive operations (full repo scan, migration parse).
	CostExpensive ResourceCost = "expensive"
)

// Concurrency and Shutdown defaults
const (
	// DefaultMaxConcurrentCheap limits parallel lightweight requests (ping, rule lookup, tools/list).
	DefaultMaxConcurrentCheap = 32
	// DefaultMaxConcurrentExpensive limits heavy static analysis / migration parsers.
	DefaultMaxConcurrentExpensive = 2
	// DefaultShutdownTimeout is the maximum duration to wait for in-flight requests during server shutdown.
	DefaultShutdownTimeout = 5 * time.Second
)

// Sentinel Errors
var (
	// ErrShutdownTimeout is returned when in-flight handlers fail to terminate within the shutdown deadline.
	ErrShutdownTimeout = errors.New("mcp: shutdown timeout exceeded: in-flight handlers did not terminate")
)

// HandlerFunc defines the execution signature for a validated JSON-RPC request.
type HandlerFunc func(ctx context.Context, req ParsedRequest) *mcperrors.JSONRPCResponse

// RequestMeta represents per-request protocol metadata in MCP 2026-07-28.
type RequestMeta struct {
	ProtocolVersion    string          `json:"io.modelcontextprotocol/protocolVersion"`
	ClientInfo         json.RawMessage `json:"io.modelcontextprotocol/clientInfo"`
	ClientCapabilities json.RawMessage `json:"io.modelcontextprotocol/clientCapabilities"`
}

// ParsedRequest represents a strictly validated JSON-RPC 2.0 request or notification.
type ParsedRequest struct {
	JSONRPC        string
	ID             any
	HasID          bool
	Method         string
	Params         json.RawMessage
	Meta           *RequestMeta // nil if request did not include _meta (legacy clients)
	IsNotification bool
}

// inFlightEntry holds the cancellation trigger and lifecycle generation for an active request.
type inFlightEntry struct {
	generation uint64
	cancel     context.CancelFunc
	cancelled  bool
}

// RequestTracker manages per-request cancellation contexts in a thread-safe manner.
type RequestTracker struct {
	mu       sync.Mutex
	nextGen  uint64
	inFlight map[string]inFlightEntry
}

// SynchronizedWriter ensures JSON-RPC responses are written sequentially to stdout without interleaving.
type SynchronizedWriter struct {
	mu sync.Mutex
	w  io.Writer
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
