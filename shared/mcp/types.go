package mcp

import (
	"github.com/will2469/argus/shared/mcp/errors"
	"github.com/will2469/argus/shared/mcp/security"
	"github.com/will2469/argus/shared/mcp/telemetry"
	"github.com/will2469/argus/shared/mcp/tools"
	"github.com/will2469/argus/shared/mcp/transport"
)

type jsonrpcResponse = errors.JSONRPCResponse
type jsonrpcError = errors.JSONRPCError

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

	CodeParseError           = errors.CodeParseError
	CodeInvalidRequest       = errors.CodeInvalidRequest
	CodeMethodNotFound       = errors.CodeMethodNotFound
	CodeInvalidParams        = errors.CodeInvalidParams
	CodeInternalError        = errors.CodeInternalError
	CodeServerNotInitialized = errors.CodeServerNotInitialized
	CodeCancelled            = errors.CodeCancelled
)

var (
	ProtocolError       = errors.ProtocolError
	InvalidParamsError  = errors.InvalidParamsError
	MethodNotFoundError = errors.MethodNotFoundError
	CancelledError      = errors.CancelledError
	ToolSuccess         = errors.ToolSuccess
	ToolError           = errors.ToolError

	ValidateJSONRPC       = transport.ValidateJSONRPC
	NewDispatcher         = transport.NewDispatcher
	NewSynchronizedWriter = transport.NewSynchronizedWriter
	NewRequestTracker     = transport.NewRequestTracker

	RegisterTool   = tools.RegisterTool
	UnregisterTool = tools.UnregisterTool

	NewPathAuthority = security.NewPathAuthority
	SubmitViaURL     = telemetry.SubmitViaURL
)
