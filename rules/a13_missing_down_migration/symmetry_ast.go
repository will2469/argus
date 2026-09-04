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

	// 1. Forward symmetry: For every DDL operation in UP, require a matching inverse in DOWN
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
