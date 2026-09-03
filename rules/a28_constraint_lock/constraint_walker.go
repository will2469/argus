// Package a28_constraint_lock validates AlterTableCmd constraint additions for zero-downtime safety (NOT VALID).
package a28_constraint_lock

import (
	"fmt"
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// InspectAlterTableConstraints checks AlterTableStmt nodes for direct table-locking constraint additions.
func InspectAlterTableConstraints(filename, content string, tree *pgquery.ParseResult, createdTables map[string]bool, dm *directives.DirectiveMap) []migration.Issue {
	var issues []migration.Issue
	if tree == nil {
		return issues
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}
		alterStmt := rawStmt.Stmt.GetAlterTableStmt()
		if alterStmt == nil || alterStmt.Relation == nil {
			continue
		}

		tblName := strings.ToLower(alterStmt.Relation.Relname)
		if createdTables[tblName] {
			continue // Safe: new table created in same migration
		}

		for _, rawCmd := range alterStmt.Cmds {
			alterCmd := rawCmd.GetAlterTableCmd()
			if alterCmd == nil {
				continue
			}

			// Focus on AT_AddConstraint
			if alterCmd.Subtype != pgquery.AlterTableType_AT_AddConstraint || alterCmd.Def == nil {
				continue
			}

			c := alterCmd.Def.GetConstraint()
			if c == nil {
				continue
			}

			// Focus on FOREIGN KEY and CHECK constraints
			isTargetType := c.Contype == pgquery.ConstrType_CONSTR_FOREIGN ||
				c.Contype == pgquery.ConstrType_CONSTR_CHECK
			if !isTargetType {
				continue
			}

			// Violation: SkipValidation is false (missing NOT VALID clause)
			if !c.SkipValidation {
				conName := c.Conname
				if conName == "" {
					conName = "unnamed"
				}

				var line int
				if conName != "unnamed" {
					line = migration.FindLineForKeyword(content, conName)
				} else {
					line = migration.FindLineFromOffset(content, int(rawStmt.StmtLocation))
				}

				if dm != nil && dm.IsLineIgnored(filename, line, RuleCode) {
					continue
				}

				typeStr := "FOREIGN KEY"
				if c.Contype == pgquery.ConstrType_CONSTR_CHECK {
					typeStr = "CHECK"
				}

				msg := fmt.Sprintf("ADD %s constraint %q on existing table %q must use NOT VALID (followed by separate VALIDATE CONSTRAINT) to prevent multi-table write lockouts", typeStr, conName, tblName)
				issues = append(issues, migration.Issue{
					Rule:     RuleCode,
					Filename: filename,
					Line:     line,
					Message:  msg,
					Severity: "CRITICAL",
				})
			}
		}
	}

	return issues
}
