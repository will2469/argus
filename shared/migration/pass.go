// Package migration provides shared scanning and diagnostic utilities for database migrations.
package migration

import (
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
)

// ResolveMigrationDir extracts the matching migration directory for the analysis pass if enabled.
// Returns the migration directory path, configuration object, and true if enabled and found.
func ResolveMigrationDir(pass *analysis.Pass, ruleCode string) (string, *config.Config, bool) {
	cfg, ok := pass.ResultOf[config.Analyzer].(*config.Config)
	if !ok || !cfg.IsRuleEnabled(ruleCode) {
		return "", nil, false
	}

	if len(pass.Files) == 0 {
		return "", cfg, false
	}

	pkgDir := filepath.Dir(pass.Fset.Position(pass.Files[0].Pos()).Filename)
	migrationDir := cfg.FindMatchingMigrationDir(pkgDir)
	if migrationDir == "" {
		return "", cfg, false
	}

	return migrationDir, cfg, true
}
