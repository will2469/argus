package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/will2469/argus/shared/directives"
)

func TestScanDirectory(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "001_test.up.sql")
	if err := os.WriteFile(file1, []byte("SELECT 1;"), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	file2 := filepath.Join(tempDir, "002_test.down.sql")
	if err := os.WriteFile(file2, []byte("SELECT 2;"), 0o600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	called := 0
	checker := func(filename, content string, dm *directives.DirectiveMap) []Issue {
		called++
		return []Issue{
			{Rule: "TEST-01", Filename: filename, Line: 1, Message: "test issue"},
		}
	}

	issues, err := ScanDirectory(tempDir, checker)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}

	if called != 1 {
		t.Errorf("expected checker to be called once for .up.sql, called %d times", called)
	}
	if len(issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(issues))
	}
}
