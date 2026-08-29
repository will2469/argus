// Package a13_missing_down_migration ensures every .up.sql migration has a valid,
// non-empty corresponding .down.sql rollback migration.
package a13_missing_down_migration

import (
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

const RuleCode = "ARGUS-A13"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A13.
var Analyzer = &analysis.Analyzer{
	Name: "a13",
	Doc:  "Ensures every .up.sql migration has a non-empty corresponding .down.sql rollback file",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	migrationDir, _, ok := migration.ResolveMigrationDir(pass, RuleCode)
	if !ok {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)
	issues := ScanDirectory(migrationDir, dm)
	for _, issue := range issues {
		pass.Reportf(pass.Files[0].Pos(), "[%s] %s: %s", RuleCode, filepath.Base(issue.Filename), issue.Message)
	}
	return nil, nil
}
