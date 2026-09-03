package transport

import (
	"bytes"
	"encoding/json"
	"strings"

	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)

// ParsedRequest represents a strictly validated JSON-RPC 2.0 request or notification.
type ParsedRequest struct {
	JSONRPC        string
	ID             any
	HasID          bool
	Method         string
	Params         json.RawMessage
	IsNotification bool
}

// ValidateJSONRPC acts as the single boundary gate for untrusted input lines.
// It enforces RFC JSON-RPC 2.0 specification conformance before dispatching.
func ValidateJSONRPC(data []byte) (*ParsedRequest, *mcperrors.JSONRPCResponse) {
	// 1. Validate JSON syntax
	if !json.Valid(data) {
		return nil, &mcperrors.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   mcperrors.JSONRPCError{Code: -32700, Message: "Parse error"},
		}
	}

	// 2. Must be a JSON object (reject arrays/batches and primitives)
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, &mcperrors.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   mcperrors.JSONRPCError{Code: -32600, Message: "Invalid Request: expected JSON object"},
		}
	}

	// 3. Extract and validate ID (required for error ID mirroring)
	var idVal any
	hasID := false
	if rawID, exists := rawMap["id"]; exists {
		hasID = true
		trimmedID := bytes.TrimSpace(rawID)
		if !bytes.Equal(trimmedID, []byte("null")) {
			var parsed any
			if err := json.Unmarshal(trimmedID, &parsed); err != nil {
				return nil, &mcperrors.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      nil,
					Error:   mcperrors.JSONRPCError{Code: -32600, Message: "Invalid Request: invalid id encoding"},
				}
			}
			switch parsed.(type) {
			case string, float64:
				idVal = parsed
			default:
				// JSON-RPC 2.0: ID MUST contain a String, Number, or NULL value if included
				return nil, &mcperrors.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      nil,
					Error:   mcperrors.JSONRPCError{Code: -32600, Message: "Invalid Request: id must be string, number, or null"},
				}
			}
		}
	}

	// 4. Validate jsonrpc version (MUST be exactly "2.0")
	rawVer, verExists := rawMap["jsonrpc"]
	if !verExists {
		return nil, &mcperrors.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      idVal,
			Error:   mcperrors.JSONRPCError{Code: -32600, Message: "Invalid Request: missing jsonrpc field"},
		}
	}
	var verStr string
	if err := json.Unmarshal(rawVer, &verStr); err != nil || verStr != "2.0" {
		return nil, &mcperrors.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      idVal,
			Error:   mcperrors.JSONRPCError{Code: -32600, Message: "Invalid Request: jsonrpc must be exactly '2.0'"},
		}
	}

	// 5. Validate method (MUST be non-empty string, no reserved rpc. prefix)
	rawMethod, methodExists := rawMap["method"]
	if !methodExists {
		return nil, &mcperrors.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      idVal,
			Error:   mcperrors.JSONRPCError{Code: -32600, Message: "Invalid Request: missing method field"},
		}
	}
	var methodStr string
	if err := json.Unmarshal(rawMethod, &methodStr); err != nil || strings.TrimSpace(methodStr) == "" {
		return nil, &mcperrors.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      idVal,
			Error:   mcperrors.JSONRPCError{Code: -32600, Message: "Invalid Request: method must be a non-empty string"},
		}
	}
	if strings.HasPrefix(methodStr, "rpc.") {
		return nil, &mcperrors.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      idVal,
			Error:   mcperrors.JSONRPCError{Code: -32600, Message: "Invalid Request: method names starting with 'rpc.' are reserved"},
		}
	}

	// 6. Validate params shape if present (MUST be structured: object or array)
	rawParams, paramsExists := rawMap["params"]
	if paramsExists {
		trimmedParams := bytes.TrimSpace(rawParams)
		if !bytes.Equal(trimmedParams, []byte("null")) {
			if len(trimmedParams) == 0 || (trimmedParams[0] != '{' && trimmedParams[0] != '[') {
				return nil, &mcperrors.JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      idVal,
					Error:   mcperrors.JSONRPCError{Code: -32600, Message: "Invalid Request: params must be structured (object or array)"},
				}
			}
		}
	}

	return &ParsedRequest{
		JSONRPC:        verStr,
		ID:             idVal,
		HasID:          hasID,
		Method:         methodStr,
		Params:         rawParams,
		IsNotification: !hasID,
	}, nil
}
