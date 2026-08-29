package migration

import (
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
)

func TestResolveMigrationDir(t *testing.T) {
	cfg := config.DefaultConfig()

	pass := &analysis.Pass{
		Fset: token.NewFileSet(),
		ResultOf: map[*analysis.Analyzer]interface{}{
			config.Analyzer: cfg,
		},
	}

	// Disabled rule
	dir, _, ok := ResolveMigrationDir(pass, "NON_EXISTENT_RULE")
	if ok || dir != "" {
		t.Errorf("expected disabled rule to return false, got ok=%v, dir=%s", ok, dir)
	}

	// Enabled rule but no files
	dir, retCfg, ok := ResolveMigrationDir(pass, "ARGUS-A11")
	if ok || dir != "" {
		t.Errorf("expected no files to return false, got ok=%v, dir=%s", ok, dir)
	}
	if retCfg == nil {
		t.Errorf("expected config to be returned")
	}
}
