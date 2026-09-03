// Package a27_concurrent_index validates that IndexStmt AST nodes on existing tables use the CONCURRENTLY flag.
package a27_concurrent_index

import (
	"fmt"
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// InspectIndexStatements checks all IndexStmt nodes in the migration AST for concurrent execution safety.
func InspectIndexStatements(filename, content string, tree *pgquery.ParseResult, createdTables map[string]bool, dm *directives.DirectiveMap) []migration.Issue {
	var issues []migration.Issue
	if tree == nil {
		return issues
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}
		indexStmt := rawStmt.Stmt.GetIndexStmt()
		if indexStmt == nil || indexStmt.Relation == nil {
			continue
		}

		tblName := strings.ToLower(indexStmt.Relation.Relname)
		// Safe: if table was created in this same migration file, it is empty and has no live traffic
		if createdTables[tblName] {
			continue
		}

		// Violation: non-concurrent index creation on existing table
		if !indexStmt.Concurrent {
			idxName := indexStmt.Idxname
			if idxName == "" {
				idxName = "unnamed"
			}

			var line int
			if idxName != "unnamed" {
				line = migration.FindLineForKeyword(content, idxName)
			} else {
				line = migration.FindLineFromOffset(content, int(rawStmt.StmtLocation))
			}

			if dm != nil && dm.IsLineIgnored(filename, line, RuleCode) {
				continue
			}

			msg := fmt.Sprintf("CREATE INDEX %q on existing table %q must use CONCURRENTLY to prevent production SHARE lockouts", idxName, tblName)
			issues = append(issues, migration.Issue{
				Rule:     RuleCode,
				Filename: filename,
				Line:     line,
				Message:  msg,
				Severity: "CRITICAL",
			})
		}
	}

	return issues
}
