// Package a05_audit_immutability inspects database migration scripts for forbidden mutations on audit tables.
package a05_audit_immutability

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// CheckMigration inspects migration SQL for tampering on audit tables.
func CheckMigration(filename, content string, dm *directives.DirectiveMap, auditTables map[string]bool) []migration.Issue {
	var issues []migration.Issue
	op, table := CheckSQLTampering(content, auditTables)
	if op != "" {
		line := migration.FindLineForKeyword(content, table)
		if dm != nil && dm.IsLineIgnored(filename, line, RuleCode) {
			return nil
		}
		msg := fmt.Sprintf("Forbidden %s on audit table %q in migration; audit trails must be strictly append-only", op, table)
		issues = append(issues, migration.Issue{
			Rule:     RuleCode,
			Filename: filename,
			Line:     line,
			Message:  msg,
			Severity: "CRITICAL",
		})
	}
	return issues
}

// InspectMigrationDir reads all .up.sql files in a migration directory and checks for tampering.
func InspectMigrationDir(migDir string, dm *directives.DirectiveMap, auditTables map[string]bool) []migration.Issue {
	entries, err := os.ReadDir(migDir)
	if err != nil {
		return nil
	}

	var allIssues []migration.Issue
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		path := filepath.Join(migDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		issues := CheckMigration(path, string(data), dm, auditTables)
		allIssues = append(allIssues, issues...)
	}
	return allIssues
}
