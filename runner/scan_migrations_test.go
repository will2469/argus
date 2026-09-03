package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/will2469/argus/shared/config"
)

func TestScanMigrations_StrictMode_ParseFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "argus_mig_strict_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	badFile := filepath.Join(tempDir, "001_corrupted.up.sql")
	if err := os.WriteFile(badFile, []byte("CREAT TABLE broken (;;;"), 0o600); err != nil {
		t.Fatalf("failed to write test migration: %v", err)
	}
	downFile := filepath.Join(tempDir, "001_corrupted.down.sql")
	if err := os.WriteFile(downFile, []byte("DROP TABLE IF EXISTS broken;"), 0o600); err != nil {
		t.Fatalf("failed to write down migration: %v", err)
	}

	tracker := NewMetricsTracker()
	cfg := config.DefaultConfig() // strict mode enabled by default

	scanMigrationDirectories([]string{tempDir}, tempDir, tracker, cfg)

	issues := tracker.issues
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue under strict mode for parse failure, got %d: %+v", len(issues), issues)
	}
	if issues[0].Rule != "ARGUS-E001" {
		t.Errorf("expected rule ARGUS-E001, got %s", issues[0].Rule)
	}
	if !strings.Contains(issues[0].Message, "PostgreSQL parser rejected statement") {
		t.Errorf("unexpected issue message: %s", issues[0].Message)
	}
}

func TestScanMigrations_PermissiveMode_ParseFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "argus_mig_permissive_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	badFile := filepath.Join(tempDir, "001_corrupted.up.sql")
	if err := os.WriteFile(badFile, []byte("CREAT TABLE broken (;;;"), 0o600); err != nil {
		t.Fatalf("failed to write test migration: %v", err)
	}
	downFile := filepath.Join(tempDir, "001_corrupted.down.sql")
	if err := os.WriteFile(downFile, []byte("DROP TABLE IF EXISTS broken;"), 0o600); err != nil {
		t.Fatalf("failed to write down migration: %v", err)
	}

	tracker := NewMetricsTracker()
	cfg := config.DefaultConfig()
	permissive := false
	cfg.Options.StrictMode = &permissive

	scanMigrationDirectories([]string{tempDir}, tempDir, tracker, cfg)

	issues := tracker.issues
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues under permissive mode, got %d: %+v", len(issues), issues)
	}
}
