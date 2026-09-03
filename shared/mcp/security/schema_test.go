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

func TestValidateSchema_IntegerSemantics(t *testing.T) {
	schema := Schema{
		Type: "object",
		Properties: map[string]Property{
			"count": {Type: "integer"},
			"score": {Type: "number"},
		},
	}

	// Valid integers (whole numbers including x.0 form)
	for _, val := range []string{"1", "0", "-5", "42", "1.0", "-3.0"} {
		err := ValidateSchema("test_tool", schema, json.RawMessage(`{"count": `+val+`}`))
		if err != nil {
			t.Errorf("expected integer %s to be accepted, got: %v", val, err)
		}
	}

	// Rejected: fractional values are NOT integers
	for _, val := range []string{"1.5", "0.1", "-2.7", "3.14"} {
		err := ValidateSchema("test_tool", schema, json.RawMessage(`{"count": `+val+`}`))
		if err == nil || !strings.Contains(err.Error(), "must be an integer") {
			t.Errorf("expected integer rejection for %s, got: %v", val, err)
		}
	}

	// Rejected: non-numeric values are NOT integers
	err := ValidateSchema("test_tool", schema, json.RawMessage(`{"count": "five"}`))
	if err == nil || !strings.Contains(err.Error(), "must be an integer") {
		t.Fatalf("expected integer rejection for string, got: %v", err)
	}
	err = ValidateSchema("test_tool", schema, json.RawMessage(`{"count": true}`))
	if err == nil || !strings.Contains(err.Error(), "must be an integer") {
		t.Fatalf("expected integer rejection for bool, got: %v", err)
	}

	// "number" type should still accept fractional values
	err = ValidateSchema("test_tool", schema, json.RawMessage(`{"score": 1.5}`))
	if err != nil {
		t.Fatalf("expected number 1.5 to be accepted for 'number' type, got: %v", err)
	}
}

func TestValidateSchema_ArrayItemsAllTypes(t *testing.T) {
	schema := Schema{
		Type: "object",
		Properties: map[string]Property{
			"flags": {Type: "array", Items: &Property{Type: "boolean"}},
			"ids":   {Type: "array", Items: &Property{Type: "integer"}},
			"vals":  {Type: "array", Items: &Property{Type: "number"}},
		},
	}

	// Boolean items: valid
	err := ValidateSchema("t", schema, json.RawMessage(`{"flags": [true, false]}`))
	if err != nil {
		t.Errorf("expected boolean array accepted, got: %v", err)
	}
	// Boolean items: rejected (string inside)
	err = ValidateSchema("t", schema, json.RawMessage(`{"flags": ["yes"]}`))
	if err == nil || !strings.Contains(err.Error(), "must be a boolean") {
		t.Errorf("expected boolean item rejection, got: %v", err)
	}

	// Integer items: valid
	err = ValidateSchema("t", schema, json.RawMessage(`{"ids": [1, 2, 3]}`))
	if err != nil {
		t.Errorf("expected integer array accepted, got: %v", err)
	}
	// Integer items: rejected (fractional)
	err = ValidateSchema("t", schema, json.RawMessage(`{"ids": [1, 2.5]}`))
	if err == nil || !strings.Contains(err.Error(), "must be an integer") {
		t.Errorf("expected integer item rejection for 2.5, got: %v", err)
	}

	// Number items: valid (fractional ok)
	err = ValidateSchema("t", schema, json.RawMessage(`{"vals": [1.5, 2.7]}`))
	if err != nil {
		t.Errorf("expected number array accepted, got: %v", err)
	}
	// Number items: rejected (string inside)
	err = ValidateSchema("t", schema, json.RawMessage(`{"vals": ["x"]}`))
	if err == nil || !strings.Contains(err.Error(), "must be a number") {
		t.Errorf("expected number item rejection for string, got: %v", err)
	}
}
