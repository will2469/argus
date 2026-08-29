// Package a29_unindexed_fk enforces that foreign key columns on child tables
// have supporting B-tree indexes where the FK column is the leading column.
package a29_unindexed_fk

import (
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

const RuleCode = "ARGUS-A29"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A29.
var Analyzer = &analysis.Analyzer{
	Name: "a29",
	Doc:  "Enforce supporting B-tree index on foreign key columns to avoid full table scans on parent deletes",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	migrationDir, cfg, ok := migration.ResolveMigrationDir(pass, RuleCode)
	if !ok {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)
	issues := ScanMigrationDir(migrationDir, dm, cfg)
	for _, issue := range issues {
		pass.Reportf(pass.Files[0].Pos(), "[%s] %s:%d: %s", RuleCode, filepath.Base(issue.Filename), issue.Line, issue.Message)
	}
	return nil, nil
}
