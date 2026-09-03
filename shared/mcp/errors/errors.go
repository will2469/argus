package errors

import "fmt"

// ProtocolError builds a top-level JSON-RPC protocol error response.
func ProtocolError(id any, code int, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   JSONRPCError{Code: code, Message: message},
	}
}

// InvalidParamsError constructs a standard -32602 JSON-RPC error.
func InvalidParamsError(id any, message string) *JSONRPCResponse {
	return ProtocolError(id, CodeInvalidParams, message)
}

// MethodNotFoundError constructs a standard -32601 JSON-RPC error.
func MethodNotFoundError(id any, method string) *JSONRPCResponse {
	return ProtocolError(id, CodeMethodNotFound, fmt.Sprintf("Method not found: %s", method))
}

// CancelledError constructs a standard -32800 JSON-RPC request cancelled error.
func CancelledError(id any, message string) *JSONRPCResponse {
	if message == "" {
		message = "Request cancelled by client"
	}
	return ProtocolError(id, CodeCancelled, message)
}

// ToolSuccess constructs a compliant MCP CallToolResult with isError: false.
func ToolSuccess(id any, text string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []TextContent{{Type: "text", Text: text}},
		},
	}
}

// ToolError constructs a compliant MCP CallToolResult with isError: true.
func ToolError(id any, text string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []TextContent{{Type: "text", Text: text}},
			"isError": true,
		},
	}
}
