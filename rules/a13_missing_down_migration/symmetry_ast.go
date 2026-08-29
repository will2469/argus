// Package a13_missing_down_migration verifies the AST and executable statements in .down.sql rollback files.
package a13_missing_down_migration

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
	"github.com/will2469/argus/shared/sqlparser"
)

// ValidateDownSQL validates that a .down.sql file contains non-empty, executable SQL statements.
func ValidateDownSQL(downPath string, content string, dm *directives.DirectiveMap) *migration.Issue {
	trimmed := strings.TrimSpace(content)
	downName := filepath.Base(downPath)

	fileDm := directives.ParseSQLDirectives(trimmed, downName)
	if fileDm != nil && fileDm.IsLineIgnored(downName, 1, RuleCode) {
		return nil
	}
	if dm != nil && dm.IsLineIgnored(downPath, 1, RuleCode) {
		return nil
	}

	if len(trimmed) == 0 {
		return &migration.Issue{
			Rule:     RuleCode,
			Filename: downPath,
			Line:     1,
			Message:  fmt.Sprintf("Rollback migration %q is empty (0 bytes); rollback requires symmetric reversal", downName),
			Severity: "HIGH",
		}
	}

	tree, err := sqlparser.Parse(trimmed)
	if err != nil || len(tree.Stmts) == 0 {
		return &migration.Issue{
			Rule:     RuleCode,
			Filename: downPath,
			Line:     1,
			Message:  fmt.Sprintf("Rollback migration %q contains no valid executable SQL statements", downName),
			Severity: "HIGH",
		}
	}

	return nil
}
