package errors

// Standard JSON-RPC 2.0 and MCP Protocol Error Codes
const (
	CodeParseError           = -32700
	CodeInvalidRequest       = -32600
	CodeMethodNotFound       = -32601
	CodeInvalidParams        = -32602
	CodeInternalError        = -32603
	CodeServerNotInitialized = -32002 // MCP and LSP standard: Server not initialized
	CodeCancelled            = -32800 // MCP RequestCancelled specification
)

// JSONRPCResponse represents an RFC-compliant JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

// JSONRPCError represents the error object inside a JSON-RPC 2.0 error response.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TextContent represents standard MCP text content payload.
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
