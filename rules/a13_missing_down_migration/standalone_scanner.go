// Package a13_missing_down_migration provides directory scanning capabilities for rollback migrations.
package a13_missing_down_migration

import (
	"os"
	"strings"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// ScanDirectory checks all migration files in a directory for missing or empty down migrations.
func ScanDirectory(dir string, dm *directives.DirectiveMap) []migration.Issue {
	var issues []migration.Issue
	entries, err := os.ReadDir(dir)
	if err != nil {
		return issues
	}

	existingFiles := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() {
			existingFiles[entry.Name()] = true
		}
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}

		upName := entry.Name()
		downPath, pairIssue := MatchPairExistence(dir, upName, existingFiles, dm)
		if pairIssue != nil {
			issues = append(issues, *pairIssue)
			continue
		}

		downData, err := os.ReadFile(downPath)
		if err != nil {
			continue
		}

		if issue := ValidateDownSQL(downPath, string(downData), dm); issue != nil {
			issues = append(issues, *issue)
		}
	}

	return issues
}
