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

// ScanDirectory walks all .up.sql files in the given directory and its subdirectories,
// aggregating issues reported by the checker.
func ScanDirectory(dir string, checker FileCheckerFunc) ([]Issue, error) {
	var allIssues []Issue
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".up.sql") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dm := directives.ParseSQLDirectives(string(data), path)
		issues := checker(path, string(data), dm)
		allIssues = append(allIssues, issues...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return allIssues, nil
}
