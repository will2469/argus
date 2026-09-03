package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
)

// ValidateSchema verifies that the raw JSON arguments conform to the declared
// Argus MCP Schema Subset (required fields, property types, enum constraints,
// array element types, and no unexpected keys).
//
// Validation coverage:
//   - Required field presence (non-null)
//   - Strict property allowlist (unexpected keys rejected)
//   - Type checking: string, boolean, number, integer, array, object
//   - Integer semantics: must be a whole number (fractional values rejected)
//   - Enum constraint: string values must match declared enum set
//   - Array items: element type validated for all supported types
func ValidateSchema(toolName string, schema Schema, rawArgs json.RawMessage) error {
	// 1. Handle empty or null arguments
	trimmed := bytes.TrimSpace(rawArgs)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		if len(schema.Required) > 0 {
			return fmt.Errorf("missing required properties: %s", strings.Join(schema.Required, ", "))
		}
		return nil
	}

	// 2. Arguments must be a JSON object
	if trimmed[0] != '{' {
		return fmt.Errorf("arguments must be a JSON object, got: %s", string(trimmed))
	}

	var argsMap map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &argsMap); err != nil {
		return fmt.Errorf("invalid arguments JSON: %w", err)
	}

	// 3. Enforce required fields
	for _, reqField := range schema.Required {
		val, exists := argsMap[reqField]
		if !exists || bytes.Equal(bytes.TrimSpace(val), []byte("null")) {
			return fmt.Errorf("missing required property %q", reqField)
		}
	}

	// 4. Disallow unknown/hallucinated properties (Strict Schema Invariant)
	for key := range argsMap {
		if _, exists := schema.Properties[key]; !exists {
			return fmt.Errorf("unexpected property %q for tool %q", key, toolName)
		}
	}

	// 5. Enforce property types and enum constraints
	for key, val := range argsMap {
		prop, exists := schema.Properties[key]
		if !exists {
			continue
		}
		if err := validateType(key, prop, bytes.TrimSpace(val)); err != nil {
			return err
		}
	}

	return nil
}

// validateType checks a single value against its declared Property type.
// This is the shared validation core used by both top-level properties and array items.
func validateType(name string, prop Property, trimmedVal []byte) error {
	if bytes.Equal(trimmedVal, []byte("null")) {
		return nil // Non-required field with explicit null is allowed
	}

	switch prop.Type {
	case "string":
		var s string
		if err := json.Unmarshal(trimmedVal, &s); err != nil {
			return fmt.Errorf("property %q must be a string", name)
		}
		if len(prop.Enum) > 0 && !slices.Contains(prop.Enum, s) {
			return fmt.Errorf("property %q must be one of [%s], got %q", name, strings.Join(prop.Enum, ", "), s)
		}
	case "boolean":
		var b bool
		if err := json.Unmarshal(trimmedVal, &b); err != nil {
			return fmt.Errorf("property %q must be a boolean", name)
		}
	case "number":
		var n float64
		if err := json.Unmarshal(trimmedVal, &n); err != nil {
			return fmt.Errorf("property %q must be a number", name)
		}
	case "integer":
		var n float64
		if err := json.Unmarshal(trimmedVal, &n); err != nil {
			return fmt.Errorf("property %q must be an integer", name)
		}
		if n != math.Floor(n) || math.IsInf(n, 0) || math.IsNaN(n) {
			return fmt.Errorf("property %q must be an integer, got %v", name, n)
		}
	case "array":
		var arr []json.RawMessage
		if err := json.Unmarshal(trimmedVal, &arr); err != nil {
			return fmt.Errorf("property %q must be an array", name)
		}
		if prop.Items != nil {
			for i, item := range arr {
				elemName := fmt.Sprintf("element %d in %q", i, name)
				if err := validateType(elemName, *prop.Items, bytes.TrimSpace(item)); err != nil {
					return err
				}
			}
		}
	case "object":
		if len(trimmedVal) == 0 || trimmedVal[0] != '{' {
			return fmt.Errorf("property %q must be an object", name)
		}
	}
	return nil
}
