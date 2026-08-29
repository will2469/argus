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
			}
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
				if cmd.Subtype == pg_query.AlterTableType_AT_DropColumn {
					reportDestructive(filename, content, rawStmt, "DROP COLUMN", &issues, dm)
				}
				if cmd.Subtype == pg_query.AlterTableType_AT_AlterColumnType {
					reportDestructive(filename, content, rawStmt, "ALTER COLUMN TYPE", &issues, dm)
				}
				if cmd.Subtype == pg_query.AlterTableType_AT_AddColumn && cmd.Def != nil {
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
							if con.Contype == pg_query.ConstrType_CONSTR_DEFAULT {
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

	return issues
}

func reportDestructive(filename, content string, rawStmt *pg_query.RawStmt, op string, issues *[]migration.Issue, dm *directives.DirectiveMap) {
	line := 1
	if rawStmt.StmtLocation > 0 {
		line = migration.FindLineFromOffset(content, int(rawStmt.StmtLocation))
	}

	if IsContractTagged(filename, content, line, dm) {
		return
	}

	*issues = append(*issues, migration.Issue{
		Rule:     RuleCode,
		Filename: filename,
		Line:     line,
		Message:  fmt.Sprintf("Destructive operation %s prohibited in .up.sql migration; apply expand-contract pattern", op),
		Severity: "CRITICAL",
	})
}
