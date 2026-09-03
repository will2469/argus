package mcp

import (
	"errors"

	errorspkg "github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
	"github.com/will2469/argus/shared/mcp/telemetry"
	"github.com/will2469/argus/shared/mcp/tools"
	"github.com/will2469/argus/shared/mcp/transport"
)

type JSONRPCResponse = errorspkg.JSONRPCResponse
type JSONRPCError = errorspkg.JSONRPCError

type jsonrpcResponse = errorspkg.JSONRPCResponse

const (
	// MaxMessageSize is the maximum allowed size (in bytes) of a single newline-delimited
	// JSON-RPC message on the stdio transport. Messages exceeding this limit cause
	// immediate connection termination — no partial execution, no partial stdout.
	MaxMessageSize = 10 * 1024 * 1024 // 10 MiB
)

// ErrOversizedMessage is returned when a client sends a message exceeding MaxMessageSize.
// This is a connection-fatal transport violation: the server terminates the connection
// without sending a JSON-RPC error response, because the message framing itself is broken.
var ErrOversizedMessage = errors.New("mcp: message exceeds maximum allowed size")

// LatestProtocolVersion is the canonical protocol version offered by Argus.
const LatestProtocolVersion = "2024-11-05"

const protocolVersion = LatestProtocolVersion

// SupportedProtocolVersions contains the set of MCP protocol versions supported by Argus.
var SupportedProtocolVersions = map[string]bool{
	"2024-11-05": true,
}

// NegotiateProtocolVersion negotiates an MCP protocol version according to the MCP specification:
// If the requested version is supported, it is selected; otherwise, the latest supported
// version is returned as the fallback.
func NegotiateProtocolVersion(requested string) string {
	if SupportedProtocolVersions[requested] {
		return requested
	}
	return LatestProtocolVersion
}

type lifecycleState uint8

const (
	statePreInit lifecycleState = iota
	stateInitializing
	stateInitialized
)

type ResourceCost = transport.ResourceCost
type ParsedRequest = transport.ParsedRequest
type Dispatcher = transport.Dispatcher
type SynchronizedWriter = transport.SynchronizedWriter
type RequestTracker = transport.RequestTracker

type Tool = tools.Tool
type ToolDef = tools.ToolDef

const (
	CostCheap     = transport.CostCheap
	CostExpensive = transport.CostExpensive

	DefaultMaxConcurrentCheap     = transport.DefaultMaxConcurrentCheap
	DefaultMaxConcurrentExpensive = transport.DefaultMaxConcurrentExpensive
	DefaultShutdownTimeout        = transport.DefaultShutdownTimeout

	CodeParseError           = errorspkg.CodeParseError
	CodeInvalidRequest       = errorspkg.CodeInvalidRequest
	CodeMethodNotFound       = errorspkg.CodeMethodNotFound
	CodeInvalidParams        = errorspkg.CodeInvalidParams
	CodeInternalError        = errorspkg.CodeInternalError
	CodeServerNotInitialized = errorspkg.CodeServerNotInitialized
	CodeCancelled            = errorspkg.CodeCancelled
)

var (
	ProtocolError       = errorspkg.ProtocolError
	InvalidParamsError  = errorspkg.InvalidParamsError
	MethodNotFoundError = errorspkg.MethodNotFoundError
	CancelledError      = errorspkg.CancelledError
	ToolSuccess         = errorspkg.ToolSuccess
	ToolError           = errorspkg.ToolError

	ValidateJSONRPC       = transport.ValidateJSONRPC
	NewDispatcher         = transport.NewDispatcher
	NewSynchronizedWriter = transport.NewSynchronizedWriter
	NewRequestTracker     = transport.NewRequestTracker
	IDKey                 = transport.IDKey
	ErrShutdownTimeout    = transport.ErrShutdownTimeout

	RegisterTool   = tools.RegisterTool
	UnregisterTool = tools.UnregisterTool

	NewPathAuthority = security.NewPathAuthority
	SubmitViaURL     = telemetry.SubmitViaURL
)
