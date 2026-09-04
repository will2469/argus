// Package a11_destructive_migration inspects PostgreSQL AST statements in migrations for destructive DDL.
package a11_destructive_migration

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
	"github.com/will2469/argus/shared/sqlparser"
)

// CheckMigration inspects a .up.sql migration for destructive DDL operations.
func CheckMigration(filename, content string, dm *directives.DirectiveMap) []migration.Issue {
	var issues []migration.Issue

	tree, err := sqlparser.Parse(content)
	if err != nil {
		return issues
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}

		// 1. DROP statements
		if dropStmt := rawStmt.Stmt.GetDropStmt(); dropStmt != nil {
			switch dropStmt.RemoveType {
			case pg_query.ObjectType_OBJECT_TABLE:
				reportDestructive(filename, content, rawStmt, "DROP TABLE", &issues, dm)
			case pg_query.ObjectType_OBJECT_COLUMN:
				reportDestructive(filename, content, rawStmt, "DROP COLUMN", &issues, dm)
			case pg_query.ObjectType_OBJECT_SCHEMA:
				reportDestructive(filename, content, rawStmt, "DROP SCHEMA", &issues, dm)
			case pg_query.ObjectType_OBJECT_DATABASE:
				reportDestructive(filename, content, rawStmt, "DROP DATABASE", &issues, dm)
			case pg_query.ObjectType_OBJECT_SEQUENCE:
				reportDestructive(filename, content, rawStmt, "DROP SEQUENCE", &issues, dm)
			case pg_query.ObjectType_OBJECT_VIEW, pg_query.ObjectType_OBJECT_MATVIEW:
				reportDestructive(filename, content, rawStmt, "DROP VIEW", &issues, dm)
			case pg_query.ObjectType_OBJECT_TYPE, pg_query.ObjectType_OBJECT_DOMAIN:
				reportDestructive(filename, content, rawStmt, "DROP TYPE", &issues, dm)
			}
		}
		if rawStmt.Stmt.GetDropdbStmt() != nil {
			reportDestructive(filename, content, rawStmt, "DROP DATABASE", &issues, dm)
		}
		if rawStmt.Stmt.GetDropTableSpaceStmt() != nil {
			reportDestructive(filename, content, rawStmt, "DROP TABLESPACE", &issues, dm)
		}
		if rawStmt.Stmt.GetDropRoleStmt() != nil {
			reportDestructive(filename, content, rawStmt, "DROP ROLE", &issues, dm)
		}

		// 2. TRUNCATE statements
		if rawStmt.Stmt.GetTruncateStmt() != nil {
			reportDestructive(filename, content, rawStmt, "TRUNCATE TABLE", &issues, dm)
		}

		// 3. RENAME statements
		if rawStmt.Stmt.GetRenameStmt() != nil {
			reportDestructive(filename, content, rawStmt, "RENAME", &issues, dm)
		}

		// 4. ALTER TABLE statements
		if alterStmt := rawStmt.Stmt.GetAlterTableStmt(); alterStmt != nil {
			for _, rawCmd := range alterStmt.Cmds {
				cmd := rawCmd.GetAlterTableCmd()
				if cmd == nil {
					continue
				}
				switch cmd.Subtype {
				case pg_query.AlterTableType_AT_DropColumn:
					reportDestructive(filename, content, rawStmt, "DROP COLUMN", &issues, dm)
				case pg_query.AlterTableType_AT_AlterColumnType:
					reportDestructive(filename, content, rawStmt, "ALTER COLUMN TYPE", &issues, dm)
				case pg_query.AlterTableType_AT_DropConstraint:
					reportDestructive(filename, content, rawStmt, "DROP CONSTRAINT", &issues, dm)
				case pg_query.AlterTableType_AT_SetNotNull:
					reportDestructive(filename, content, rawStmt, "ALTER COLUMN SET NOT NULL", &issues, dm)
				case pg_query.AlterTableType_AT_DetachPartition, pg_query.AlterTableType_AT_DetachPartitionFinalize:
					reportDestructive(filename, content, rawStmt, "DETACH PARTITION", &issues, dm)
				case pg_query.AlterTableType_AT_AddColumn:
					if cmd.Def != nil {
						if colDef := cmd.Def.GetColumnDef(); colDef != nil {
							hasNotNull := false
							hasDefault := false
							for _, c := range colDef.Constraints {
								con := c.GetConstraint()
								if con == nil {
									continue
								}
								if con.Contype == pg_query.ConstrType_CONSTR_NOTNULL {
									hasNotNull = true
								}
								if con.Contype == pg_query.ConstrType_CONSTR_DEFAULT || con.Contype == pg_query.ConstrType_CONSTR_GENERATED {
									hasDefault = true
								}
							}
							if hasNotNull && !hasDefault {
								reportDestructive(filename, content, rawStmt, "ADD COLUMN NOT NULL without DEFAULT", &issues, dm)
							}
						}
					}
				}
			}
		}
	}

	return issues
}

func reportDestructive(filename, content string, rawStmt *pg_query.RawStmt, op string, issues *[]migration.Issue, dm *directives.DirectiveMap) {
	line := 1
	if rawStmt.StmtLocation > 0 {
		line = migration.FindLineFromOffset(content, int(rawStmt.StmtLocation))
	}

	// 1. Check administrative ignore directive
	if dm != nil && dm.IsLineIgnored(filename, line, RuleCode) {
		return
	}

	// 2. Extract migration phase metadata & validate contract evidence
	meta := ExtractMigrationPhaseMetadata(filename, content, line)
	res := ValidateContractEvidence(meta)
	if res.IsValid {
		return
	}

	msg := fmt.Sprintf("Destructive operation %s prohibited in .up.sql migration; apply expand-contract pattern", op)
	if res.Reason != "" && res.Reason != "no contract phase declared" {
		msg = fmt.Sprintf("Destructive operation %s prohibited in .up.sql migration; %s", op, res.Reason)
	}

	*issues = append(*issues, migration.Issue{
		Rule:     RuleCode,
		Filename: filename,
		Line:     line,
		Message:  msg,
		Severity: "CRITICAL",
	})
}
