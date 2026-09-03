package security

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateSchema_RequiredFields(t *testing.T) {
	schema := Schema{
		Type: "object",
		Properties: map[string]Property{
			"sql": {Type: "string"},
		},
		Required: []string{"sql"},
	}

	// Missing required field
	err := ValidateSchema("test_tool", schema, json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), `missing required property "sql"`) {
		t.Fatalf("expected missing required property error, got: %v", err)
	}

	// Explicit null
	err = ValidateSchema("test_tool", schema, json.RawMessage(`{"sql": null}`))
	if err == nil || !strings.Contains(err.Error(), `missing required property "sql"`) {
		t.Fatalf("expected missing required property error on null, got: %v", err)
	}
}

func TestValidateSchema_UnexpectedProperties(t *testing.T) {
	schema := Schema{
		Type: "object",
		Properties: map[string]Property{
			"sql": {Type: "string"},
		},
	}

	err := ValidateSchema("test_tool", schema, json.RawMessage(`{"sql": "SELECT 1", "extra": true}`))
	if err == nil || !strings.Contains(err.Error(), `unexpected property "extra"`) {
		t.Fatalf("expected unexpected property error, got: %v", err)
	}
}

func TestValidateSchema_TypeMismatches(t *testing.T) {
	schema := Schema{
		Type: "object",
		Properties: map[string]Property{
			"dirs": {
				Type:  "array",
				Items: &Property{Type: "string"},
			},
			"count": {Type: "number"},
		},
	}

	// Array type mismatch
	err := ValidateSchema("test_tool", schema, json.RawMessage(`{"dirs": "not-an-array"}`))
	if err == nil || !strings.Contains(err.Error(), `must be an array`) {
		t.Fatalf("expected array type error, got: %v", err)
	}

	// Element type mismatch
	err = ValidateSchema("test_tool", schema, json.RawMessage(`{"dirs": [123]}`))
	if err == nil || !strings.Contains(err.Error(), `must be a string`) {
		t.Fatalf("expected element string error, got: %v", err)
	}
}
