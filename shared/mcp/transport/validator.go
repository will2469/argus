package transport

import (
	"bytes"
	"encoding/json"
	"strings"

	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)


// ValidateJSONRPC acts as the single boundary gate for untrusted input lines.
// It enforces RFC JSON-RPC 2.0 specification conformance before dispatching.
func ValidateJSONRPC(data []byte) (*ParsedRequest, *mcperrors.JSONRPCResponse) {
	// 1. Validate JSON syntax
	if !json.Valid(data) {
		return nil, mcperrors.ProtocolError(nil, mcperrors.CodeParseError, "Parse error")
	}

	// 2. Reject duplicate top-level keys (parser differential / request smuggling defense).
	// Go's encoding/json silently takes the last value for duplicates, which creates
	// ambiguity between parsers that take first vs last. Reject deterministically.
	if hasDuplicateKeys(data) {
		return nil, mcperrors.ProtocolError(nil, mcperrors.CodeInvalidRequest, "Invalid Request: duplicate keys in JSON object")
	}

	// 3. Must be a JSON object (reject arrays/batches and primitives)
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return nil, mcperrors.ProtocolError(nil, mcperrors.CodeInvalidRequest, "Invalid Request: expected JSON object")
	}

	// 4. Extract and validate ID (required for error ID mirroring)
	var idVal any
	hasID := false
	if rawID, exists := rawMap["id"]; exists {
		hasID = true
		trimmedID := bytes.TrimSpace(rawID)
		if !bytes.Equal(trimmedID, []byte("null")) {
			var parsed any
			if err := json.Unmarshal(trimmedID, &parsed); err != nil {
				return nil, mcperrors.ProtocolError(nil, mcperrors.CodeInvalidRequest, "Invalid Request: invalid id encoding")
			}
			switch parsed.(type) {
			case string, float64:
				idVal = parsed
			default:
				// JSON-RPC 2.0: ID MUST contain a String, Number, or NULL value if included
				return nil, mcperrors.ProtocolError(nil, mcperrors.CodeInvalidRequest, "Invalid Request: id must be string, number, or null")
			}
		}
	}

	// 5. Validate jsonrpc version (MUST be exactly "2.0")
	rawVer, verExists := rawMap["jsonrpc"]
	if !verExists {
		return nil, mcperrors.ProtocolError(idVal, mcperrors.CodeInvalidRequest, "Invalid Request: missing jsonrpc field")
	}
	var verStr string
	if err := json.Unmarshal(rawVer, &verStr); err != nil || verStr != "2.0" {
		return nil, mcperrors.ProtocolError(idVal, mcperrors.CodeInvalidRequest, "Invalid Request: jsonrpc must be exactly '2.0'")
	}

	// 6. Validate method (MUST be non-empty string, no reserved rpc. prefix)
	rawMethod, methodExists := rawMap["method"]
	if !methodExists {
		return nil, mcperrors.ProtocolError(idVal, mcperrors.CodeInvalidRequest, "Invalid Request: missing method field")
	}
	var methodStr string
	if err := json.Unmarshal(rawMethod, &methodStr); err != nil || strings.TrimSpace(methodStr) == "" {
		return nil, mcperrors.ProtocolError(idVal, mcperrors.CodeInvalidRequest, "Invalid Request: method must be a non-empty string")
	}
	if strings.HasPrefix(methodStr, "rpc.") {
		return nil, mcperrors.ProtocolError(idVal, mcperrors.CodeInvalidRequest, "Invalid Request: method names starting with 'rpc.' are reserved")
	}

	// 7. Validate params shape if present (MUST be structured: object or array)
	var meta *RequestMeta
	rawParams, paramsExists := rawMap["params"]
	if paramsExists {
		trimmedParams := bytes.TrimSpace(rawParams)
		if !bytes.Equal(trimmedParams, []byte("null")) {
			if len(trimmedParams) == 0 || (trimmedParams[0] != '{' && trimmedParams[0] != '[') {
				return nil, mcperrors.ProtocolError(idVal, mcperrors.CodeInvalidRequest, "Invalid Request: params must be structured (object or array)")
			}
			if trimmedParams[0] == '{' {
				// We parse _meta with strict precedence:
				// 1. Primary / Official Spec (MCP 2026-07-28): "io.modelcontextprotocol/protocolVersion"
				// 2. Permissive Fallback: unnamespaced "protocolVersion" for compatibility with early/draft clients.
				var withMeta struct {
					Meta struct {
						ProtocolVersion    string          `json:"io.modelcontextprotocol/protocolVersion"`
						FallbackVersion    string          `json:"protocolVersion"`
						ClientInfo         json.RawMessage `json:"io.modelcontextprotocol/clientInfo"`
						ClientCapabilities json.RawMessage `json:"io.modelcontextprotocol/clientCapabilities"`
					} `json:"_meta"`
				}
				if err := json.Unmarshal(trimmedParams, &withMeta); err == nil {
					v := withMeta.Meta.ProtocolVersion
					if v == "" {
						v = withMeta.Meta.FallbackVersion
					}
					if v != "" || len(withMeta.Meta.ClientInfo) > 0 || len(withMeta.Meta.ClientCapabilities) > 0 {
						meta = &RequestMeta{
							ProtocolVersion:    v,
							ClientInfo:         withMeta.Meta.ClientInfo,
							ClientCapabilities: withMeta.Meta.ClientCapabilities,
						}
					}
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
		Meta:           meta,
		IsNotification: !hasID,
	}, nil
}

// hasDuplicateKeys walks the top-level JSON object tokens using json.Decoder
// and returns true if any key appears more than once. This runs BEFORE
// json.Unmarshal (which silently deduplicates), closing the parser-differential
// ambiguity window that enables request smuggling.
func hasDuplicateKeys(data []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(data))

	// Consume opening token; must be '{' for an object.
	tok, err := dec.Token()
	if err != nil {
		return false
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return false // Not an object; let downstream validation handle it.
	}

	seen := make(map[string]struct{})
	for dec.More() {
		// Read key token.
		keyTok, err := dec.Token()
		if err != nil {
			return false
		}
		key, ok := keyTok.(string)
		if !ok {
			return false
		}
		if _, exists := seen[key]; exists {
			return true // Duplicate found!
		}
		seen[key] = struct{}{}

		// Skip the value (may be nested object/array — we only check top-level).
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return false
		}
	}
	return false
}
