// Package a28_constraint_lock enforces 2-phase zero-downtime constraint additions
// (NOT VALID followed by VALIDATE CONSTRAINT) on existing tables in migrations.
package a28_constraint_lock

import (
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
	"github.com/will2469/argus/shared/sqlparser"
)

const RuleCode = "ARGUS-A28"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A28.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a28_table_locking_constraint_addition",
	Doc:  "Enforce 2-phase zero-downtime constraint addition (NOT VALID followed by VALIDATE CONSTRAINT) on existing tables",
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

// CheckMigration evaluates a single migration file for direct table-locking constraint additions.
func CheckMigration(filename, content string, dm *directives.DirectiveMap) []migration.Issue {
	tree, err := sqlparser.Parse(content)
	if err != nil {
		return nil
	}

	createdTables := CollectCreatedTables(tree)
	return InspectAlterTableConstraints(filename, content, tree, createdTables, dm)
}
