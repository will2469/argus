package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallFile(t *testing.T) {
	tempDir := t.TempDir()
	dummyBin := filepath.Join(tempDir, "argus")

	if err := os.WriteFile(dummyBin, []byte("binary-content"), 0755); err != nil {
		t.Fatalf("failed to create dummy binary: %v", err)
	}

	if err := uninstallFile(dummyBin); err != nil {
		t.Fatalf("uninstallFile failed: %v", err)
	}

	if _, err := os.Stat(dummyBin); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted, but it still exists")
	}
}
