// Package migration provides shared scanning and diagnostic utilities for database migrations.
package migration

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/will2469/argus/shared/directives"
)

// FileCheckerFunc defines the function signature for per-file migration scanners.
type FileCheckerFunc func(filename, content string, dm *directives.DirectiveMap) []Issue

// ScanDirectory walks all .up.sql files in the given directory and aggregates issues reported by the checker.
func ScanDirectory(dir string, checker FileCheckerFunc) ([]Issue, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var allIssues []Issue
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		dm := directives.ParseSQLDirectives(string(data), filePath)
		issues := checker(filePath, string(data), dm)
		allIssues = append(allIssues, issues...)
	}

	return allIssues, nil
}
