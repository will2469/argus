// Package a13_missing_down_migration tracks schema state across sequential migrations.
package a13_missing_down_migration

import (
	"strings"
)

// SchemaCatalog tracks table and column definitions across sequential migrations.
type SchemaCatalog struct {
	columns map[QualifiedIdent]map[string]string // qualified table identifier -> column name -> type
}

// NewSchemaCatalog creates a new empty schema catalog.
func NewSchemaCatalog() *SchemaCatalog {
	return &SchemaCatalog{
		columns: make(map[QualifiedIdent]map[string]string),
	}
}

// Clone creates a deep copy of the schema catalog.
func (sc *SchemaCatalog) Clone() *SchemaCatalog {
	if sc == nil {
		return nil
	}
	clone := NewSchemaCatalog()
	for tbl, cols := range sc.columns {
		clone.columns[tbl] = make(map[string]string, len(cols))
		for col, typ := range cols {
			clone.columns[tbl][col] = typ
		}
	}
	return clone
}

// FormatQualifiedName returns a canonical "schema.table" identifier.
// Unqualified names default to the "public" schema.
func FormatQualifiedName(schema, rel string) string {
	rel = strings.ToLower(strings.Trim(strings.TrimSpace(rel), `"'`))
	schema = strings.ToLower(strings.Trim(strings.TrimSpace(schema), `"'`))

	if idx := strings.Index(rel, "."); idx != -1 {
		if schema == "" {
			schema = rel[:idx]
		}
		rel = rel[idx+1:]
	}

	if schema == "" || schema == "public" {
		schema = "public"
	}
	return schema + "." + rel
}

// DisplayQualifiedName formats a canonical name for user diagnostics.
func DisplayQualifiedName(name string) string {
	if strings.HasPrefix(name, "public.") {
		return strings.TrimPrefix(name, "public.")
	}
	return name
}

// NormalizeTypeName canonicalizes PostgreSQL type names and aliases.
func NormalizeTypeName(tName string) string {
	tName = strings.ToLower(strings.TrimSpace(tName))
	switch tName {
	case "int4", "integer", "int", "serial":
		return "integer"
	case "int8", "bigint", "bigserial":
		return "bigint"
	case "int2", "smallint", "smallserial":
		return "smallint"
	case "bool", "boolean":
		return "boolean"
	case "float8", "double precision":
		return "double precision"
	case "float4", "real":
		return "real"
	case "varchar", "character varying":
		return "varchar"
	default:
		return tName
	}
}

// GetColumnType retrieves the recorded type for a table and column.
func (sc *SchemaCatalog) GetColumnType(table QualifiedIdent, column string) (string, bool) {
	if sc == nil {
		return "", false
	}
	col := normalizeCol(column)
	cols, exists := sc.columns[table]
	if !exists {
		return "", false
	}
	t, ok := cols[col]
	return t, ok
}

// SetColumnType registers or updates a column type in the catalog.
func (sc *SchemaCatalog) SetColumnType(table QualifiedIdent, column, colType string) {
	if sc == nil {
		return
	}
	col := normalizeCol(column)
	if sc.columns[table] == nil {
		sc.columns[table] = make(map[string]string)
	}
	sc.columns[table][col] = NormalizeTypeName(colType)
}

// DropColumn removes a column from the catalog.
func (sc *SchemaCatalog) DropColumn(table QualifiedIdent, column string) {
	if sc == nil {
		return
	}
	col := normalizeCol(column)
	if cols, exists := sc.columns[table]; exists {
		delete(cols, col)
	}
}

// DropTable removes an entire table from the catalog.
func (sc *SchemaCatalog) DropTable(table QualifiedIdent) {
	if sc == nil {
		return
	}
	delete(sc.columns, table)
}

// ApplyOps applies forward schema mutations from an UP migration.
func (sc *SchemaCatalog) ApplyOps(ops []SchemaOp) {
	if sc == nil {
		return
	}
	for _, op := range ops {
		switch op.Kind {
		case OpCreateTable:
			for col, typ := range op.ColDefs {
				sc.SetColumnType(op.Target, col, typ)
			}
		case OpDropTable:
			sc.DropTable(op.Target)
		case OpAddColumn:
			sc.SetColumnType(op.Target, op.SubTarget, op.AuxTarget)
		case OpDropColumn:
			sc.DropColumn(op.Target, op.SubTarget)
		case OpAlterColumnType:
			sc.SetColumnType(op.Target, op.SubTarget, op.AuxTarget)
		case OpRenameTable:
			if cols, exists := sc.columns[op.Target]; exists {
				sc.columns[op.NewTable] = cols
				delete(sc.columns, op.Target)
			}
		case OpRenameColumn:
			oldCol := normalizeCol(op.SubTarget)
			newCol := normalizeCol(op.AuxTarget)
			if cols, exists := sc.columns[op.Target]; exists {
				if typ, ok := cols[oldCol]; ok {
					cols[newCol] = typ
					delete(cols, oldCol)
				}
			}
		}
	}
}
