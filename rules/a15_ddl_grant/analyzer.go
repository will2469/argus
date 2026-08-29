// Package a15_ddl_grant forbids granting DDL permissions or table ownership
// to runtime application roles in migration scripts.
package a15_ddl_grant

import (
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// RuleCode is the official identifier for ARGUS-A15.
const RuleCode = "ARGUS-A15"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A15.
var Analyzer = &analysis.Analyzer{
	Name: "a15",
	Doc:  "Forbids granting DDL permissions (CREATE, TRUNCATE, ALL PRIVILEGES) or table ownership to runtime app roles",
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

	reg := FromConfig(cfg)
	issues := ScanDirectory(migrationDir, reg)
	for _, issue := range issues {
		pass.Reportf(pass.Files[0].Pos(), "[%s] %s: %s", RuleCode, filepath.Base(issue.Filename), issue.Message)
	}

	return nil, nil
}
