// Package a30_timestamptz scans table definitions and alter commands for bare TIMESTAMP columns.
package a30_timestamptz

import (
	"fmt"
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// InspectTableStatements checks all CREATE TABLE and ALTER TABLE statements in the migration AST.
func InspectTableStatements(filename, content string, tree *pgquery.ParseResult, dm *directives.DirectiveMap) []migration.Issue {
	var issues []migration.Issue
	if tree == nil {
		return issues
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}

		// 1. Inspect CREATE TABLE
		if createStmt := rawStmt.Stmt.GetCreateStmt(); createStmt != nil && createStmt.Relation != nil {
			tbl := strings.ToLower(createStmt.Relation.Relname)
			for _, elt := range createStmt.TableElts {
				if colDef := elt.GetColumnDef(); colDef != nil {
					if IsBareTimestamp(colDef.TypeName) {
						colName := strings.ToLower(colDef.Colname)
						line := migration.FindLineForKeyword(content, colName)
						if dm != nil && dm.IsLineIgnored(filename, line, RuleCode) {
							continue
						}
						msg := fmt.Sprintf("Column %q on table %q uses bare TIMESTAMP without time zone; use TIMESTAMPTZ for UTC determinism", colName, tbl)
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
		}

		// 2. Inspect ALTER TABLE (ADD COLUMN / ALTER TYPE)
		if alterStmt := rawStmt.Stmt.GetAlterTableStmt(); alterStmt != nil && alterStmt.Relation != nil {
			tbl := strings.ToLower(alterStmt.Relation.Relname)
			for _, rawCmd := range alterStmt.Cmds {
				cmd := rawCmd.GetAlterTableCmd()
				if cmd != nil && cmd.Def != nil {
					if colDef := cmd.Def.GetColumnDef(); colDef != nil {
						if IsBareTimestamp(colDef.TypeName) {
							colName := strings.ToLower(colDef.Colname)
							line := migration.FindLineForKeyword(content, colName)
							if dm != nil && dm.IsLineIgnored(filename, line, RuleCode) {
								continue
							}
							msg := fmt.Sprintf("Column %q on table %q uses bare TIMESTAMP without time zone; use TIMESTAMPTZ for UTC determinism", colName, tbl)
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
			}
		}
	}

	return issues
}
