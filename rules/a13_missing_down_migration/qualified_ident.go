// Package a13_missing_down_migration provides database-qualified object identifiers.
package a13_missing_down_migration

import (
	"strings"
)

// QualifiedIdent represents a database-qualified object identifier.
type QualifiedIdent struct {
	Schema string
	Name   string
}

// NewQualifiedIdent creates a normalized qualified identifier.
func NewQualifiedIdent(schema, name string) QualifiedIdent {
	schema = strings.ToLower(strings.Trim(strings.TrimSpace(schema), `"'`))
	name = strings.ToLower(strings.Trim(strings.TrimSpace(name), `"'`))

	if idx := strings.Index(name, "."); idx != -1 {
		if schema == "" {
			schema = name[:idx]
		}
		name = name[idx+1:]
	}

	if schema == "" || schema == "public" {
		schema = "public"
	}
	return QualifiedIdent{
		Schema: schema,
		Name:   name,
	}
}

// NewSchemaIdent creates an identifier representing a database schema.
func NewSchemaIdent(name string) QualifiedIdent {
	name = strings.ToLower(strings.Trim(strings.TrimSpace(name), `"'`))
	return QualifiedIdent{
		Schema: "",
		Name:   name,
	}
}

// Equal checks whether two qualified identifiers are identical.
func (q QualifiedIdent) Equal(other QualifiedIdent) bool {
	return q.Schema == other.Schema && q.Name == other.Name
}

// String formats the qualified name canonically as "schema.name".
func (q QualifiedIdent) String() string {
	if q.Schema == "" || q.Schema == "public" {
		return "public." + q.Name
	}
	return q.Schema + "." + q.Name
}

// Display formats the name for user-facing diagnostics.
func (q QualifiedIdent) Display() string {
	if q.Schema == "" || q.Schema == "public" {
		return q.Name
	}
	return q.Schema + "." + q.Name
}

// IsEmpty reports whether the identifier has no name.
func (q QualifiedIdent) IsEmpty() bool {
	return q.Name == ""
}
