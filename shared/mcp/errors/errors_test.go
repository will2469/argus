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
	resSuccess := parsed.Result.(map[string]any)
	if resSuccess["resultType"] != "complete" {
		t.Fatalf("expected resultType=complete, got %v", resSuccess["resultType"])
	}
	metaSuccess, ok := resSuccess["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("expected _meta in tool success result, got %v", resSuccess)
	}
	sInfo, ok := metaSuccess["io.modelcontextprotocol/serverInfo"].(map[string]any)
	if !ok || sInfo["name"] != "argus" {
		t.Fatalf("expected serverInfo name=argus, got: %v", sInfo)
	}

	// Tool Error
	toolErr := ToolError("req-2", "failed")
	dataErr, _ := json.Marshal(toolErr)
	var parsedErr JSONRPCResponse
	_ = json.Unmarshal(dataErr, &parsedErr)
	resMap := parsedErr.Result.(map[string]any)
	if resMap["resultType"] != "complete" {
		t.Fatalf("expected resultType=complete, got %v", resMap["resultType"])
	}
	if isErr, _ := resMap["isError"].(bool); !isErr {
		t.Fatal("tool error must have isError: true")
	}
}
