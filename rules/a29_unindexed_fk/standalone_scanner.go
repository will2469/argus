// Package a29_unindexed_fk provides scanner routines for migration directories.
package a29_unindexed_fk

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
	"github.com/will2469/argus/shared/sqlparser"
)

// ScanMigrationDir scans all migration files in a directory and subdirectories and cross-references FKs with Indexes.
func ScanMigrationDir(dir string, dm *directives.DirectiveMap, cfg *config.Config) []migration.Issue {
	files := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".up.sql") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			files[path] = string(data)
			if dm != nil {
				fileDm := directives.ParseSQLDirectives(string(data), path)
				lineCount := strings.Count(string(data), "\n") + 1
				for l := 1; l <= lineCount; l++ {
					if fileDm.IsLineIgnored(path, l, RuleCode) {
						dm.AddDirective(path, l, RuleCode, "sql directive")
						dm.AddDirective(info.Name(), l, RuleCode, "sql directive")
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil
	}
	return CheckMigrations(files, dm, cfg)
}

// CheckMigrations evaluates multiple migration files together to verify foreign key indexing.
func CheckMigrations(files map[string]string, dm *directives.DirectiveMap, cfg *config.Config) []migration.Issue {
	graph := NewSchemaGraph()

	var filenames []string
	for fn := range files {
		filenames = append(filenames, fn)
	}
	sort.Strings(filenames)

	for _, fn := range filenames {
		content := files[fn]
		tree, err := sqlparser.Parse(content)
		if err != nil {
			continue
		}
		graph.CollectFromTree(fn, content, tree)
	}

	return graph.MatchUnindexedForeignKeys(dm, cfg)
}
