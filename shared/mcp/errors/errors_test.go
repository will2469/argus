package errors

import (
	"encoding/json"
	"testing"
)

func TestErrorSemantics(t *testing.T) {
	// Protocol Error
	protoErr := ProtocolError(1, CodeInvalidRequest, "invalid")
	if protoErr.ID != 1 {
		t.Fatalf("expected id=1, got %v", protoErr.ID)
	}
	if protoErr.Error == nil {
		t.Fatal("expected error to be non-nil")
	}

	// Tool Success
	success := ToolSuccess("req-1", "ok")
	data, _ := json.Marshal(success)
	var parsed JSONRPCResponse
	_ = json.Unmarshal(data, &parsed)
	if parsed.Error != nil {
		t.Fatal("tool success must have nil error envelope")
	}

	// Tool Error
	toolErr := ToolError("req-2", "failed")
	dataErr, _ := json.Marshal(toolErr)
	var parsedErr JSONRPCResponse
	_ = json.Unmarshal(dataErr, &parsedErr)
	resMap := parsedErr.Result.(map[string]any)
	if isErr, _ := resMap["isError"].(bool); !isErr {
		t.Fatal("tool error must have isError: true")
	}
}
