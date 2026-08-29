// Package a11_destructive_migration enforces that .up.sql schema migrations do not execute
// destructive DDL operations that break zero-downtime rolling deployments.
package a11_destructive_migration

import (
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

const RuleCode = "ARGUS-A11"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A11.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a11_destructive_migration",
	Doc:  "Forbids destructive DDL operations (DROP COLUMN, DROP TABLE, TRUNCATE, RENAME, ALTER TYPE) in .up.sql migrations",
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
		pass.Reportf(pass.Files[0].Pos(), "[%s] %s: %s", RuleCode, filepath.Base(issue.Filename), issue.Message)
	}

	return nil, nil
}
