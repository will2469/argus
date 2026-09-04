// Package a13_missing_down_migration verifies the AST and executable statements in .down.sql rollback files.
package a13_missing_down_migration

import (
	"fmt"
	"path/filepath"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
	"github.com/will2469/argus/shared/sqlparser"
)

// ValidateDownSQL validates that a .down.sql file contains non-empty, executable SQL statements
// that semantically invert the schema operations in the corresponding .up.sql file.
func ValidateDownSQL(upPath, upContent, downPath, downContent string, dm *directives.DirectiveMap) *migration.Issue {
	trimmedDown := strings.TrimSpace(downContent)
	downName := filepath.Base(downPath)
	upName := filepath.Base(upPath)

	// 1. Directives suppression check
	fileDm := directives.ParseSQLDirectives(trimmedDown, downName)
	if fileDm != nil && fileDm.IsLineIgnored(downName, 1, RuleCode) {
		return nil
	}
	if dm != nil && (dm.IsLineIgnored(downPath, 1, RuleCode) || dm.IsLineIgnored(upPath, 1, RuleCode)) {
		return nil
	}
	if upContent != "" {
		upDm := directives.ParseSQLDirectives(upContent, upName)
		if upDm != nil && upDm.IsLineIgnored(upName, 1, RuleCode) {
			return nil
		}
	}

	// 2. Empty / 0-byte check
	if len(trimmedDown) == 0 {
		return &migration.Issue{
			Rule:     RuleCode,
			Filename: downPath,
			Line:     1,
			Message:  fmt.Sprintf("Rollback migration %q is empty (0 bytes); rollback requires symmetric reversal", downName),
			Severity: "HIGH",
		}
	}

	// 3. Parse DOWN SQL
	downTree, err := sqlparser.Parse(trimmedDown)
	if err != nil || len(downTree.Stmts) == 0 {
		return &migration.Issue{
			Rule:     RuleCode,
			Filename: downPath,
			Line:     1,
			Message:  fmt.Sprintf("Rollback migration %q contains no valid executable SQL statements", downName),
			Severity: "HIGH",
		}
	}

	// 4. Parse UP SQL and check semantic rollback symmetry
	trimmedUp := strings.TrimSpace(upContent)
	if len(trimmedUp) == 0 {
		return nil
	}
	upTree, err := sqlparser.Parse(trimmedUp)
	if err != nil {
		return nil
	}

	upOps := extractSchemaOps(upTree)
	downOps := extractSchemaOps(downTree)

	return checkSymmetry(upPath, downPath, upOps, downOps)
}

func checkSymmetry(upPath, downPath string, upOps, downOps []SchemaOp) *migration.Issue {
	downName := filepath.Base(downPath)
	upName := filepath.Base(upPath)

	// For every DDL operation in UP, require a matching inverse in DOWN
	for _, upOp := range upOps {
		if upOp.Kind == OpDML {
			continue
		}
		matched := false
		for _, downOp := range downOps {
			if upOp.IsInvertedBy(downOp) {
				matched = true
				break
			}
		}
		if !matched {
			return &migration.Issue{
				Rule:     RuleCode,
				Filename: downPath,
				Line:     1,
				Message:  fmt.Sprintf("Rollback migration %q is not a valid inverse for %q: missing %s for %s", downName, upName, upOp.ExpectedInverseName(), upOp.DescribeTarget()),
				Severity: "HIGH",
			}
		}
	}

	// 2. Backward symmetry: Every DDL operation in DOWN must invert an operation in UP
	for _, downOp := range downOps {
		if downOp.Kind == OpDML {
			continue
		}
		invertsAny := false
		for _, upOp := range upOps {
			if upOp.IsInvertedBy(downOp) {
				invertsAny = true
				break
			}
		}
		if !invertsAny {
			return &migration.Issue{
				Rule:     RuleCode,
				Filename: downPath,
				Line:     1,
				Message:  fmt.Sprintf("Rollback migration %q contains unexpected schema mutation on %s with no corresponding operation in %q", downName, downOp.DescribeTarget(), upName),
				Severity: "HIGH",
			}
		}
	}

	// 3. If UP and DOWN contain only DML without any DDL inverse and no suppression directive:
	if !hasDDL(upOps) && !hasDDL(downOps) && len(downOps) > 0 {
		return &migration.Issue{
			Rule:     RuleCode,
			Filename: downPath,
			Line:     1,
			Message:  fmt.Sprintf("Rollback migration %q contains no inverse operations for %q; use '-- argus:ignore-a13 ADR-xxx <reason>' if intentionally irreversible", downName, upName),
			Severity: "HIGH",
		}
	}

	return nil
}

func hasDDL(ops []SchemaOp) bool {
	for _, op := range ops {
		if op.Kind != OpDML {
			return true
		}
	}
	return false
}

func extractSchemaOps(tree *pg_query.ParseResult) []SchemaOp {
	var ops []SchemaOp
	if tree == nil {
		return ops
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}
		stmt := rawStmt.Stmt

		if create := stmt.GetCreateStmt(); create != nil && create.Relation != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateTable, Target: create.Relation.Relname})
			continue
		}

		if drop := stmt.GetDropStmt(); drop != nil {
			for _, obj := range drop.Objects {
				name := extractObjectName(obj)
				switch drop.RemoveType {
				case pg_query.ObjectType_OBJECT_TABLE:
					ops = append(ops, SchemaOp{Kind: OpDropTable, Target: name})
				case pg_query.ObjectType_OBJECT_INDEX:
					ops = append(ops, SchemaOp{Kind: OpDropIndex, Target: name})
				case pg_query.ObjectType_OBJECT_VIEW, pg_query.ObjectType_OBJECT_MATVIEW:
					ops = append(ops, SchemaOp{Kind: OpDropView, Target: name})
				case pg_query.ObjectType_OBJECT_SEQUENCE:
					ops = append(ops, SchemaOp{Kind: OpDropSequence, Target: name})
				case pg_query.ObjectType_OBJECT_SCHEMA:
					ops = append(ops, SchemaOp{Kind: OpDropSchema, Target: name})
				case pg_query.ObjectType_OBJECT_TYPE, pg_query.ObjectType_OBJECT_DOMAIN:
					ops = append(ops, SchemaOp{Kind: OpDropType, Target: name})
				}
			}
			continue
		}

		if idx := stmt.GetIndexStmt(); idx != nil {
			tbl := ""
			if idx.Relation != nil {
				tbl = idx.Relation.Relname
			}
			ops = append(ops, SchemaOp{Kind: OpCreateIndex, Target: idx.Idxname, SubTarget: tbl})
			continue
		}

		if view := stmt.GetViewStmt(); view != nil && view.View != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateView, Target: view.View.Relname})
			continue
		}

		if seq := stmt.GetCreateSeqStmt(); seq != nil && seq.Sequence != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateSequence, Target: seq.Sequence.Relname})
			continue
		}

		if sch := stmt.GetCreateSchemaStmt(); sch != nil {
			ops = append(ops, SchemaOp{Kind: OpCreateSchema, Target: sch.Schemaname})
			continue
		}

		if alter := stmt.GetAlterTableStmt(); alter != nil && alter.Relation != nil {
			tbl := alter.Relation.Relname
			for _, rawCmd := range alter.Cmds {
				cmd := rawCmd.GetAlterTableCmd()
				if cmd == nil {
					continue
				}
				switch cmd.Subtype {
				case pg_query.AlterTableType_AT_AddColumn:
					if cmd.Def != nil && cmd.Def.GetColumnDef() != nil {
						ops = append(ops, SchemaOp{Kind: OpAddColumn, Target: tbl, SubTarget: cmd.Def.GetColumnDef().Colname})
					}
				case pg_query.AlterTableType_AT_DropColumn:
					ops = append(ops, SchemaOp{Kind: OpDropColumn, Target: tbl, SubTarget: cmd.Name})
				case pg_query.AlterTableType_AT_AddConstraint:
					con := ""
					if cmd.Def != nil && cmd.Def.GetConstraint() != nil {
						con = cmd.Def.GetConstraint().Conname
					}
					ops = append(ops, SchemaOp{Kind: OpAddConstraint, Target: tbl, SubTarget: con})
				case pg_query.AlterTableType_AT_DropConstraint:
					ops = append(ops, SchemaOp{Kind: OpDropConstraint, Target: tbl, SubTarget: cmd.Name})
				}
			}
			continue
		}

		if stmt.GetSelectStmt() != nil || stmt.GetUpdateStmt() != nil || stmt.GetInsertStmt() != nil || stmt.GetDeleteStmt() != nil {
			ops = append(ops, SchemaOp{Kind: OpDML})
		}
	}

	return ops
}
