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

// ScanMigrationDir scans all migration files in a directory and cross-references FKs with Indexes.
func ScanMigrationDir(dir string, dm *directives.DirectiveMap, cfg *config.Config) []migration.Issue {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	files := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err == nil {
			files[entry.Name()] = string(data)
			if dm != nil {
				fileDm := directives.ParseSQLDirectives(string(data), entry.Name())
				for l := 1; l <= strings.Count(string(data), "\n")+1; l++ {
					if fileDm.IsLineIgnored(entry.Name(), l, RuleCode) {
						dm.AddDirective(entry.Name(), l, RuleCode, "sql directive")
					}
				}
			}
		}
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
