package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Schema represents JSON Schema definition for tool inputs.
type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property defines a schema property attribute.
type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
	Items       *Property `json:"items,omitempty"`
}

// ValidateSchema verifies that the raw JSON arguments strictly conform to
// the declared tool input schema (required fields, property types, and no unexpected keys).
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
		trimmedVal := bytes.TrimSpace(val)
		if bytes.Equal(trimmedVal, []byte("null")) {
			continue // Non-required field with explicit null is allowed
		}

		switch prop.Type {
		case "string":
			var s string
			if err := json.Unmarshal(trimmedVal, &s); err != nil {
				return fmt.Errorf("property %q must be a string", key)
			}
			if len(prop.Enum) > 0 && !slices.Contains(prop.Enum, s) {
				return fmt.Errorf("property %q must be one of [%s], got %q", key, strings.Join(prop.Enum, ", "), s)
			}
		case "boolean":
			var b bool
			if err := json.Unmarshal(trimmedVal, &b); err != nil {
				return fmt.Errorf("property %q must be a boolean", key)
			}
		case "number", "integer":
			var n float64
			if err := json.Unmarshal(trimmedVal, &n); err != nil {
				return fmt.Errorf("property %q must be a number", key)
			}
		case "array":
			var arr []json.RawMessage
			if err := json.Unmarshal(trimmedVal, &arr); err != nil {
				return fmt.Errorf("property %q must be an array", key)
			}
			if prop.Items != nil {
				for i, item := range arr {
					trimmedItem := bytes.TrimSpace(item)
					switch prop.Items.Type {
					case "string":
						var itemStr string
						if err := json.Unmarshal(trimmedItem, &itemStr); err != nil {
							return fmt.Errorf("element %d in %q must be a string", i, key)
						}
					}
				}
			}
		case "object":
			if len(trimmedVal) == 0 || trimmedVal[0] != '{' {
				return fmt.Errorf("property %q must be an object", key)
			}
		}
	}

	return nil
}
