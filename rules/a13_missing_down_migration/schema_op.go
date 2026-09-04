// Package a13_missing_down_migration extracts schema operations and checks rollback symmetry.
package a13_missing_down_migration

import (
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
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
	OpDML
)

// SchemaOp represents a detected database schema operation.
type SchemaOp struct {
	Kind      OpKind
	Target    string // Primary object name (table, index, view, etc.)
	SubTarget string // Secondary object name (column, constraint, etc.)
}

// IsInvertedBy checks if downOp is the inverse operation of this schema op.
func (op SchemaOp) IsInvertedBy(downOp SchemaOp) bool {
	tUp := normalizeName(op.Target)
	tDown := normalizeName(downOp.Target)
	sUp := normalizeName(op.SubTarget)
	sDown := normalizeName(downOp.SubTarget)

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
		if downOp.Kind == OpDropTable && tUp != "" && tUp == tDown {
			return true // Dropping the table also reverts any created index on it
		}
		if downOp.Kind == OpDropIndex {
			return tUp == tDown || op.Target == "" || downOp.Target == ""
		}
		return false
	case OpDropIndex:
		return downOp.Kind == OpCreateIndex && (tUp == tDown || op.Target == "" || downOp.Target == "")
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
		return downOp.Kind == OpDropConstraint && tUp == tDown && (sUp == sDown || sUp == "" || sDown == "")
	case OpDropConstraint:
		return downOp.Kind == OpAddConstraint && tUp == tDown && (sUp == sDown || sUp == "" || sDown == "")
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
	default:
		return "inverse operation"
	}
}

// DescribeTarget returns a human-readable description of the target object.
func (op SchemaOp) DescribeTarget() string {
	switch op.Kind {
	case OpCreateTable, OpDropTable:
		return fmt.Sprintf("table %q", op.Target)
	case OpAddColumn, OpDropColumn:
		return fmt.Sprintf("column %q on table %q", op.SubTarget, op.Target)
	case OpCreateIndex, OpDropIndex:
		return fmt.Sprintf("index %q", op.Target)
	case OpCreateView, OpDropView:
		return fmt.Sprintf("view %q", op.Target)
	case OpCreateSequence, OpDropSequence:
		return fmt.Sprintf("sequence %q", op.Target)
	case OpCreateSchema, OpDropSchema:
		return fmt.Sprintf("schema %q", op.Target)
	case OpCreateType, OpDropType:
		return fmt.Sprintf("type %q", op.Target)
	case OpAddConstraint, OpDropConstraint:
		return fmt.Sprintf("constraint %q on table %q", op.SubTarget, op.Target)
	default:
		return fmt.Sprintf("object %q", op.Target)
	}
}

func extractObjectName(obj *pg_query.Node) string {
	if obj == nil {
		return ""
	}
	if list := obj.GetList(); list != nil {
		var parts []string
		for _, item := range list.Items {
			if s := item.GetString_(); s != nil {
				parts = append(parts, s.Sval)
			}
		}
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	if str := obj.GetString_(); str != nil {
		return str.Sval
	}
	return ""
}

func normalizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, `"'`)
	name = strings.ToLower(name)
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}
	return name
}
