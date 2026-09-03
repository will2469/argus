// Package a13_missing_down_migration provides directory scanning capabilities for rollback migrations.
package a13_missing_down_migration

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// ScanDirectory checks all migration files in a directory and subdirectories for missing or empty down migrations.
func ScanDirectory(dir string, dm *directives.DirectiveMap) []migration.Issue {
	var issues []migration.Issue

	dirFiles := make(map[string]map[string]bool)
	dirUpFiles := make(map[string][]string)

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		if !strings.HasSuffix(name, ".sql") {
			return nil
		}
		d := filepath.Dir(path)
		if dirFiles[d] == nil {
			dirFiles[d] = make(map[string]bool)
		}
		dirFiles[d][name] = true
		if strings.HasSuffix(name, ".up.sql") {
			dirUpFiles[d] = append(dirUpFiles[d], name)
		}
		return nil
	})

	var dirs []string
	for d := range dirUpFiles {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	for _, d := range dirs {
		upNames := dirUpFiles[d]
		sort.Strings(upNames)
		existingFiles := dirFiles[d]
		for _, upName := range upNames {
			downPath, pairIssue := MatchPairExistence(d, upName, existingFiles, dm)
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
	}

	return issues
}
