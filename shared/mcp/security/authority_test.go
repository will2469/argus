package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathAuthority_SymlinkEscape(t *testing.T) {
	allowedRoot := t.TempDir()
	outsideDir := t.TempDir()
	secretFile := filepath.Join(outsideDir, "secret.go")
	if err := os.WriteFile(secretFile, []byte("package secret"), 0644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	symlinkPath := filepath.Join(allowedRoot, "vendor_symlink")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	auth, err := NewPathAuthority(allowedRoot)
	if err != nil {
		t.Fatalf("failed to create PathAuthority: %v", err)
	}

	_, err = auth.ValidatePath(symlinkPath)
	if err == nil || !strings.Contains(err.Error(), "path authority violation") {
		t.Fatalf("expected symlink escape to be rejected, got: %v", err)
	}

	symlinkTargetFile := filepath.Join(symlinkPath, "secret.go")
	_, err = auth.ValidatePath(symlinkTargetFile)
	if err == nil || !strings.Contains(err.Error(), "path authority violation") {
		t.Fatalf("expected target file via symlink to be rejected, got: %v", err)
	}

	symlinkPhantomFile := filepath.Join(symlinkPath, "nested", "phantom.go")
	_, err = auth.ValidatePath(symlinkPhantomFile)
	if err == nil || !strings.Contains(err.Error(), "path authority violation") {
		t.Fatalf("expected non-existent child via symlink to be rejected, got: %v", err)
	}
}

func TestPathAuthority_MultipleAllowedRoots(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	unauthorizedRoot := t.TempDir()

	auth, err := NewPathAuthority(rootA, rootB)
	if err != nil {
		t.Fatalf("failed to create PathAuthority: %v", err)
	}

	if _, err := auth.ValidatePath(filepath.Join(rootA, "pkg1")); err != nil {
		t.Errorf("expected rootA path to be valid, got: %v", err)
	}
	if _, err := auth.ValidatePath(filepath.Join(rootB, "pkg2")); err != nil {
		t.Errorf("expected rootB path to be valid, got: %v", err)
	}

	_, err = auth.ValidatePath(filepath.Join(unauthorizedRoot, "pkg3"))
	if err == nil || !strings.Contains(err.Error(), "path authority violation") {
		t.Errorf("expected unauthorized root path to be rejected, got: %v", err)
	}
}

func TestPathAuthority_StandardTraversal(t *testing.T) {
	root := t.TempDir()
	auth, err := NewPathAuthority(root)
	if err != nil {
		t.Fatalf("failed to create PathAuthority: %v", err)
	}

	_, err = auth.ValidatePath(filepath.Join(root, "../../etc"))
	if err == nil || !strings.Contains(err.Error(), "path authority violation") {
		t.Fatalf("expected .. traversal to be rejected, got: %v", err)
	}

	_, err = auth.ValidatePath("")
	if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("expected empty path to be rejected, got: %v", err)
	}

	validChild := filepath.Join(root, "valid_child")
	if _, err := auth.ValidatePath(validChild); err != nil {
		t.Fatalf("expected valid child to be accepted, got: %v", err)
	}
}
