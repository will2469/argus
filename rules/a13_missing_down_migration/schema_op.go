// Package a13_missing_down_migration extracts schema operations and checks rollback symmetry.
package a13_missing_down_migration

import (
	"fmt"
	"strings"
)

// OpKind represents a schema DDL or DML operation kind.
type OpKind int

const (
	OpCreateTable OpKind = iota
	OpDropTable
	OpAddColumn
	OpDropColumn
	OpCreateIndex
	OpDropIndex
	OpCreateView
	OpDropView
	OpCreateSequence
	OpDropSequence
	OpCreateSchema
	OpDropSchema
	OpCreateType
	OpDropType
	OpAddConstraint
	OpDropConstraint
	OpRenameTable
	OpRenameColumn
	OpAlterColumnType
	OpDML
)

// SchemaOp represents a detected database schema operation.
type SchemaOp struct {
	Kind      OpKind
	Target    string            // Primary object name (table, index, view, etc.)
	SubTarget string            // Secondary object name (column, constraint, or old/target name)
	AuxTarget string            // Tertiary object name (new name or new type)
	ColDefs   map[string]string // Column definitions for table creation
}

// IsInvertedBy checks if downOp is the inverse operation of this schema op.
func (op SchemaOp) IsInvertedBy(downOp SchemaOp) bool {
	return op.IsInvertedByWithCatalog(downOp, nil)
}

// IsInvertedByWithCatalog checks if downOp is the inverse operation with catalog context.
func (op SchemaOp) IsInvertedByWithCatalog(downOp SchemaOp, cat *SchemaCatalog) bool {
	tUp := normalizeCanonical(op.Target)
	tDown := normalizeCanonical(downOp.Target)
	sUp := normalizeCanonical(op.SubTarget)
	sDown := normalizeCanonical(downOp.SubTarget)
	aUp := normalizeCanonical(op.AuxTarget)
	aDown := normalizeCanonical(downOp.AuxTarget)

	switch op.Kind {
	case OpCreateTable:
		return downOp.Kind == OpDropTable && tUp == tDown
	case OpDropTable:
		return downOp.Kind == OpCreateTable && tUp == tDown
	case OpAddColumn:
		if downOp.Kind == OpDropTable && tUp == tDown {
			return true // Dropping the table also reverts any added column
		}
		return downOp.Kind == OpDropColumn && tUp == tDown && sUp == sDown
	case OpDropColumn:
		return downOp.Kind == OpAddColumn && tUp == tDown && sUp == sDown
	case OpCreateIndex:
		if downOp.Kind == OpDropTable && sUp != "" && sUp == tDown {
			return true // Dropping the table also reverts any created index on it
		}
		if downOp.Kind == OpDropIndex {
			return tUp == tDown || (op.Target == "" && downOp.Target == "")
		}
		return false
	case OpDropIndex:
		return downOp.Kind == OpCreateIndex && (tUp == tDown || (op.Target == "" && downOp.Target == ""))
	case OpCreateView:
		return downOp.Kind == OpDropView && tUp == tDown
	case OpDropView:
		return downOp.Kind == OpCreateView && tUp == tDown
	case OpCreateSequence:
		return downOp.Kind == OpDropSequence && tUp == tDown
	case OpDropSequence:
		return downOp.Kind == OpCreateSequence && tUp == tDown
	case OpCreateSchema:
		return downOp.Kind == OpDropSchema && tUp == tDown
	case OpDropSchema:
		return downOp.Kind == OpCreateSchema && tUp == tDown
	case OpCreateType:
		return downOp.Kind == OpDropType && tUp == tDown
	case OpDropType:
		return downOp.Kind == OpCreateType && tUp == tDown
	case OpAddConstraint:
		if downOp.Kind == OpDropTable && tUp == tDown {
			return true
		}
		return downOp.Kind == OpDropConstraint && tUp == tDown && sUp == sDown
	case OpDropConstraint:
		return downOp.Kind == OpAddConstraint && tUp == tDown && sUp == sDown
	case OpRenameTable:
		if downOp.Kind == OpDropTable && (tDown == sUp || tDown == tUp) {
			return true
		}
		return downOp.Kind == OpRenameTable && tUp == sDown && sUp == tDown
	case OpRenameColumn:
		if downOp.Kind == OpDropTable && tUp == tDown {
			return true
		}
		if downOp.Kind == OpDropColumn && tUp == tDown && (sDown == aUp || sDown == sUp) {
			return true
		}
		return downOp.Kind == OpRenameColumn && tUp == tDown && sUp == aDown && aUp == sDown
	case OpAlterColumnType:
		if downOp.Kind == OpDropTable && tUp == tDown {
			return true
		}
		if downOp.Kind != OpAlterColumnType || tUp != tDown || sUp != sDown {
			return false
		}
		// Strict rejection: identical type cannot undo alteration
		if aUp != "" && aDown != "" && aUp == aDown {
			return false
		}
		// Proven inverse: down type must match original type recorded in catalog
		if cat != nil {
			if origType, ok := cat.GetColumnType(op.Target, op.SubTarget); ok {
				return aDown == origType
			}
		}
		return false
	default:
		return false
	}
}

// ExpectedInverseName returns the name of the operation needed to undo this op.
func (op SchemaOp) ExpectedInverseName() string {
	switch op.Kind {
	case OpCreateTable:
		return "DROP TABLE"
	case OpDropTable:
		return "CREATE TABLE"
	case OpAddColumn:
		return "DROP COLUMN"
	case OpDropColumn:
		return "ADD COLUMN"
	case OpCreateIndex:
		return "DROP INDEX"
	case OpDropIndex:
		return "CREATE INDEX"
	case OpCreateView:
		return "DROP VIEW"
	case OpDropView:
		return "CREATE VIEW"
	case OpCreateSequence:
		return "DROP SEQUENCE"
	case OpDropSequence:
		return "CREATE SEQUENCE"
	case OpCreateSchema:
		return "DROP SCHEMA"
	case OpDropSchema:
		return "CREATE SCHEMA"
	case OpCreateType:
		return "DROP TYPE"
	case OpDropType:
		return "CREATE TYPE"
	case OpAddConstraint:
		return "DROP CONSTRAINT"
	case OpDropConstraint:
		return "ADD CONSTRAINT"
	case OpRenameTable:
		return fmt.Sprintf("RENAME TABLE %q TO %q", DisplayQualifiedName(op.SubTarget), DisplayQualifiedName(op.Target))
	case OpRenameColumn:
		return fmt.Sprintf("RENAME COLUMN %q TO %q", op.AuxTarget, op.SubTarget)
	case OpAlterColumnType:
		return "ALTER COLUMN TYPE"
	default:
		return "inverse operation"
	}
}

// DescribeTarget returns a human-readable description of the target object.
func (op SchemaOp) DescribeTarget() string {
	switch op.Kind {
	case OpCreateTable, OpDropTable:
		return fmt.Sprintf("table %q", DisplayQualifiedName(op.Target))
	case OpAddColumn, OpDropColumn:
		return fmt.Sprintf("column %q on table %q", op.SubTarget, DisplayQualifiedName(op.Target))
	case OpCreateIndex, OpDropIndex:
		return fmt.Sprintf("index %q", DisplayQualifiedName(op.Target))
	case OpCreateView, OpDropView:
		return fmt.Sprintf("view %q", DisplayQualifiedName(op.Target))
	case OpCreateSequence, OpDropSequence:
		return fmt.Sprintf("sequence %q", DisplayQualifiedName(op.Target))
	case OpCreateSchema, OpDropSchema:
		return fmt.Sprintf("schema %q", op.Target)
	case OpCreateType, OpDropType:
		return fmt.Sprintf("type %q", DisplayQualifiedName(op.Target))
	case OpAddConstraint, OpDropConstraint:
		return fmt.Sprintf("constraint %q on table %q", op.SubTarget, DisplayQualifiedName(op.Target))
	case OpRenameTable:
		return fmt.Sprintf("renamed table %q to %q", DisplayQualifiedName(op.Target), DisplayQualifiedName(op.SubTarget))
	case OpRenameColumn:
		return fmt.Sprintf("renamed column %q to %q on table %q", op.SubTarget, op.AuxTarget, DisplayQualifiedName(op.Target))
	case OpAlterColumnType:
		return fmt.Sprintf("type alteration on column %q of table %q", op.SubTarget, DisplayQualifiedName(op.Target))
	default:
		return fmt.Sprintf("object %q", DisplayQualifiedName(op.Target))
	}
}

func normalizeCanonical(name string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(name), `"'`))
}
