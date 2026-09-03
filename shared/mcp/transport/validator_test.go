package transport

import (
	"testing"

	mcperrors "github.com/will2469/argus/shared/mcp/errors"
)

func TestValidateJSONRPC_ValidRequests(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		expectedID any
		hasID      bool
		isNotif    bool
		method     string
	}{
		{
			name:       "valid request with int id",
			input:      `{"jsonrpc":"2.0","id":1,"method":"ping"}`,
			expectedID: float64(1),
			hasID:      true,
			isNotif:    false,
			method:     "ping",
		},
		{
			name:       "valid request with string id",
			input:      `{"jsonrpc":"2.0","id":"req-99","method":"tools/list"}`,
			expectedID: "req-99",
			hasID:      true,
			isNotif:    false,
			method:     "tools/list",
		},
		{
			name:       "valid request with object params",
			input:      `{"jsonrpc":"2.0","id":2,"method":"test","params":{"foo":"bar"}}`,
			expectedID: float64(2),
			hasID:      true,
			isNotif:    false,
			method:     "test",
		},
		{
			name:       "valid request with array params",
			input:      `{"jsonrpc":"2.0","id":3,"method":"test","params":[1,2,3]}`,
			expectedID: float64(3),
			hasID:      true,
			isNotif:    false,
			method:     "test",
		},
		{
			name:       "valid notification without id",
			input:      `{"jsonrpc":"2.0","method":"notifications/initialized"}`,
			expectedID: nil,
			hasID:      false,
			isNotif:    true,
			method:     "notifications/initialized",
		},
		{
			name:       "valid request with explicit null id",
			input:      `{"jsonrpc":"2.0","id":null,"method":"ping"}`,
			expectedID: nil,
			hasID:      true,
			isNotif:    false,
			method:     "ping",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, errResp := ValidateJSONRPC([]byte(tc.input))
			if errResp != nil {
				t.Fatalf("expected valid request, got error: %+v", errResp.Error)
			}
			if req.Method != tc.method {
				t.Errorf("expected method %q, got %q", tc.method, req.Method)
			}
			if req.HasID != tc.hasID {
				t.Errorf("expected hasID %v, got %v", tc.hasID, req.HasID)
			}
			if req.IsNotification != tc.isNotif {
				t.Errorf("expected isNotif %v, got %v", tc.isNotif, req.IsNotification)
			}
			if req.ID != tc.expectedID {
				t.Errorf("expected ID %v, got %v", tc.expectedID, req.ID)
			}
		})
	}
}

func TestValidateJSONRPC_Rejections(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedCode int
		expectedID   any
	}{
		{
			name:         "malformed JSON syntax",
			input:        `{"jsonrpc":`,
			expectedCode: -32700,
			expectedID:   nil,
		},
		{
			name:         "array payload (batch unsupported)",
			input:        `[{"jsonrpc":"2.0","id":1,"method":"ping"}]`,
			expectedCode: -32600,
			expectedID:   nil,
		},
		{
			name:         "primitive payload string",
			input:        `"hello"`,
			expectedCode: -32600,
			expectedID:   nil,
		},
		{
			name:         "missing jsonrpc field",
			input:        `{"id":1,"method":"ping"}`,
			expectedCode: -32600,
			expectedID:   float64(1),
		},
		{
			name:         "invalid jsonrpc version 1.0",
			input:        `{"jsonrpc":"1.0","id":2,"method":"ping"}`,
			expectedCode: -32600,
			expectedID:   float64(2),
		},
		{
			name:         "numeric jsonrpc version",
			input:        `{"jsonrpc":2.0,"id":3,"method":"ping"}`,
			expectedCode: -32600,
			expectedID:   float64(3),
		},
		{
			name:         "missing method field",
			input:        `{"jsonrpc":"2.0","id":4}`,
			expectedCode: -32600,
			expectedID:   float64(4),
		},
		{
			name:         "empty method string",
			input:        `{"jsonrpc":"2.0","id":5,"method":"  "}`,
			expectedCode: -32600,
			expectedID:   float64(5),
		},
		{
			name:         "reserved rpc. method prefix",
			input:        `{"jsonrpc":"2.0","id":6,"method":"rpc.internal"}`,
			expectedCode: -32600,
			expectedID:   float64(6),
		},
		{
			name:         "boolean id type is illegal",
			input:        `{"jsonrpc":"2.0","id":true,"method":"ping"}`,
			expectedCode: -32600,
			expectedID:   nil,
		},
		{
			name:         "object id type is illegal",
			input:        `{"jsonrpc":"2.0","id":{"k":"v"},"method":"ping"}`,
			expectedCode: -32600,
			expectedID:   nil,
		},
		{
			name:         "primitive params string is illegal",
			input:        `{"jsonrpc":"2.0","id":7,"method":"ping","params":"not-an-object"}`,
			expectedCode: -32600,
			expectedID:   float64(7),
		},
		{
			name:         "primitive params number is illegal",
			input:        `{"jsonrpc":"2.0","id":8,"method":"ping","params":12345}`,
			expectedCode: -32600,
			expectedID:   float64(8),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, errResp := ValidateJSONRPC([]byte(tc.input))
			if req != nil {
				t.Fatalf("expected request to be rejected, but got parsed request: %+v", req)
			}
			if errResp == nil {
				t.Fatal("expected error response, got nil")
			}
			errObj, ok := errResp.Error.(mcperrors.JSONRPCError)
			if !ok {
				t.Fatalf("expected JSONRPCError, got %T", errResp.Error)
			}
			if errObj.Code != tc.expectedCode {
				t.Errorf("expected error code %d, got %d (msg: %s)", tc.expectedCode, errObj.Code, errObj.Message)
			}
			if errResp.ID != tc.expectedID {
				t.Errorf("expected mirrored id %v, got %v", tc.expectedID, errResp.ID)
			}
		})
	}
}
