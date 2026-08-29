// Package a27_concurrent_index enforces that creating indexes on existing tables
// in migration scripts must use the CREATE INDEX CONCURRENTLY syntax.
package a27_concurrent_index

import (
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
	"github.com/will2469/argus/shared/sqlparser"
)

const RuleCode = "ARGUS-A27"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A27.
var Analyzer = &analysis.Analyzer{
	Name: "a27",
	Doc:  "Enforce CREATE INDEX CONCURRENTLY on existing tables in migrations to prevent production write outages",
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

// CheckMigration evaluates a single migration file for non-concurrent index creations.
func CheckMigration(filename, content string, dm *directives.DirectiveMap) []migration.Issue {
	tree, err := sqlparser.Parse(content)
	if err != nil {
		return nil // Skip unparseable statements
	}

	createdTables := CollectCreatedTables(tree)
	return InspectIndexStatements(filename, content, tree, createdTables, dm)
}
