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
	Target    QualifiedIdent    // Primary object (table, index, view, etc.)
	Table     QualifiedIdent    // Parent table (for index, column, constraint)
	SubTarget string            // Secondary object name (column, constraint)
	AuxTarget string            // Tertiary object name (new name or new type)
	NewTable  QualifiedIdent    // For OpRenameTable: target table
	ColDefs   map[string]string // Column definitions for table creation
}

// IsInvertedBy checks if downOp is the inverse operation of this schema op.
func (op SchemaOp) IsInvertedBy(downOp SchemaOp) bool {
	return op.IsInvertedByWithCatalog(downOp, nil)
}

// IsInvertedByWithCatalog checks if downOp is the inverse operation with catalog context.
func (op SchemaOp) IsInvertedByWithCatalog(downOp SchemaOp, cat *SchemaCatalog) bool {
	switch op.Kind {
	case OpCreateTable:
		return downOp.Kind == OpDropTable && op.Target.Equal(downOp.Target)
	case OpDropTable:
		return downOp.Kind == OpCreateTable && op.Target.Equal(downOp.Target)
	case OpAddColumn:
		return downOp.Kind == OpDropColumn &&
			op.Target.Equal(downOp.Target) &&
			normalizeCol(op.SubTarget) == normalizeCol(downOp.SubTarget)
	case OpDropColumn:
		if downOp.Kind != OpAddColumn || !op.Target.Equal(downOp.Target) || normalizeCol(op.SubTarget) != normalizeCol(downOp.SubTarget) {
			return false
		}
		if cat != nil && downOp.AuxTarget != "" {
			if origType, ok := cat.GetColumnType(op.Target, op.SubTarget); ok {
				return NormalizeTypeName(downOp.AuxTarget) == origType
			}
		}
		return true
	case OpCreateIndex:
		return downOp.Kind == OpDropIndex &&
			(op.Target.Equal(downOp.Target) || (op.Target.IsEmpty() && downOp.Target.IsEmpty()))
	case OpDropIndex:
		return downOp.Kind == OpCreateIndex &&
			(op.Target.Equal(downOp.Target) || (op.Target.IsEmpty() && downOp.Target.IsEmpty()))
	case OpCreateView:
		return downOp.Kind == OpDropView && op.Target.Equal(downOp.Target)
	case OpDropView:
		return downOp.Kind == OpCreateView && op.Target.Equal(downOp.Target)
	case OpCreateSequence:
		return downOp.Kind == OpDropSequence && op.Target.Equal(downOp.Target)
	case OpDropSequence:
		return downOp.Kind == OpCreateSequence && op.Target.Equal(downOp.Target)
	case OpCreateSchema:
		return downOp.Kind == OpDropSchema && op.Target.Equal(downOp.Target)
	case OpDropSchema:
		return downOp.Kind == OpCreateSchema && op.Target.Equal(downOp.Target)
	case OpCreateType:
		return downOp.Kind == OpDropType && op.Target.Equal(downOp.Target)
	case OpDropType:
		return downOp.Kind == OpCreateType && op.Target.Equal(downOp.Target)
	case OpAddConstraint:
		return downOp.Kind == OpDropConstraint &&
			op.Target.Equal(downOp.Target) &&
			normalizeCol(op.SubTarget) == normalizeCol(downOp.SubTarget)
	case OpDropConstraint:
		return downOp.Kind == OpAddConstraint &&
			op.Target.Equal(downOp.Target) &&
			normalizeCol(op.SubTarget) == normalizeCol(downOp.SubTarget)
	case OpRenameTable:
		return downOp.Kind == OpRenameTable &&
			op.Target.Equal(downOp.NewTable) &&
			op.NewTable.Equal(downOp.Target)
	case OpRenameColumn:
		return downOp.Kind == OpRenameColumn &&
			op.Target.Equal(downOp.Target) &&
			normalizeCol(op.SubTarget) == normalizeCol(downOp.AuxTarget) &&
			normalizeCol(op.AuxTarget) == normalizeCol(downOp.SubTarget)
	case OpAlterColumnType:
		if downOp.Kind != OpAlterColumnType || !op.Target.Equal(downOp.Target) || normalizeCol(op.SubTarget) != normalizeCol(downOp.SubTarget) {
			return false
		}
		aUp := NormalizeTypeName(op.AuxTarget)
		aDown := NormalizeTypeName(downOp.AuxTarget)
		if aUp != "" && aDown != "" && aUp == aDown {
			return false
		}
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
		return fmt.Sprintf("RENAME TABLE %q TO %q", op.NewTable.Display(), op.Target.Display())
	case OpRenameColumn:
		return fmt.Sprintf("RENAME COLUMN %q TO %q on table %q", op.AuxTarget, op.SubTarget, op.Target.Display())
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
		return fmt.Sprintf("table %q", op.Target.Display())
	case OpAddColumn, OpDropColumn:
		return fmt.Sprintf("column %q on table %q", op.SubTarget, op.Target.Display())
	case OpCreateIndex, OpDropIndex:
		return fmt.Sprintf("index %q", op.Target.Display())
	case OpCreateView, OpDropView:
		return fmt.Sprintf("view %q", op.Target.Display())
	case OpCreateSequence, OpDropSequence:
		return fmt.Sprintf("sequence %q", op.Target.Display())
	case OpCreateSchema, OpDropSchema:
		return fmt.Sprintf("schema %q", op.Target.Name)
	case OpCreateType, OpDropType:
		return fmt.Sprintf("type %q", op.Target.Display())
	case OpAddConstraint, OpDropConstraint:
		return fmt.Sprintf("constraint %q on table %q", op.SubTarget, op.Target.Display())
	case OpRenameTable:
		return fmt.Sprintf("renamed table %q to %q", op.Target.Display(), op.NewTable.Display())
	case OpRenameColumn:
		return fmt.Sprintf("renamed column %q to %q on table %q", op.SubTarget, op.AuxTarget, op.Target.Display())
	case OpAlterColumnType:
		return fmt.Sprintf("type alteration on column %q of table %q", op.SubTarget, op.Target.Display())
	default:
		return fmt.Sprintf("object %q", op.Target.Display())
	}
}

func normalizeCol(name string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(name), `"'`))
}
