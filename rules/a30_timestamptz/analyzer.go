// Package a30_timestamptz enforces that temporal columns must use TIMESTAMPTZ
// (timestamp with time zone) instead of bare TIMESTAMP (without time zone).
package a30_timestamptz

import (
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
	"github.com/will2469/argus/shared/sqlparser"
)

const RuleCode = "ARGUS-A30"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A30.
var Analyzer = &analysis.Analyzer{
	Name: "a30",
	Doc:  "Enforce TIMESTAMPTZ over bare TIMESTAMP to guarantee UTC chronological data integrity",
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

	issues, err := ScanMigrationDir(migrationDir)
	if err != nil {
		return nil, nil
	}

	for _, issue := range issues {
		pass.Reportf(pass.Files[0].Pos(), "[%s] %s:%d: %s", RuleCode, filepath.Base(issue.Filename), issue.Line, issue.Message)
	}
	return nil, nil
}

// CheckMigration inspects table column definitions in a migration file for bare TIMESTAMP.
func CheckMigration(filename, content string, dm *directives.DirectiveMap) []migration.Issue {
	tree, err := sqlparser.Parse(content)
	if err != nil {
		return nil
	}

	return InspectTableStatements(filename, content, tree, dm)
}
