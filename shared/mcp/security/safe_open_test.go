package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeOpen_KernelContainment(t *testing.T) {
	root := t.TempDir()

	// Create a valid file inside the root
	validFile := filepath.Join(root, "valid.txt")
	if err := os.WriteFile(validFile, []byte("safe content"), 0644); err != nil {
		t.Fatalf("failed to write valid file: %v", err)
	}

	auth, err := NewPathAuthority(root)
	if err != nil {
		t.Fatalf("failed to create PathAuthority: %v", err)
	}

	// 1. SafeOpen succeeds for valid file
	f, err := auth.SafeOpen("valid.txt")
	if err != nil {
		t.Fatalf("expected SafeOpen to succeed for valid.txt, got: %v", err)
	}
	f.Close()

	// 2. SafeOpen rejects directory traversal
	_, err = auth.SafeOpen("../../etc/passwd")
	if err == nil {
		t.Fatal("expected SafeOpen to reject traversal, got nil error")
	}

	// 3. SafeOpen rejects symlink escape
	outsideDir := t.TempDir()
	secretFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	symlinkPath := filepath.Join(root, "escape_link")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	_, err = auth.SafeOpen("escape_link/secret.txt")
	if err == nil {
		t.Fatal("expected SafeOpen to reject symlink escape, got nil error")
	}
}

func TestSafeOpen_TOCTOURaceDefense(t *testing.T) {
	root := t.TempDir()

	// Create a safe target file
	safeDir := filepath.Join(root, "data")
	if err := os.Mkdir(safeDir, 0755); err != nil {
		t.Fatalf("failed to create safe dir: %v", err)
	}
	safeFile := filepath.Join(safeDir, "file.txt")
	if err := os.WriteFile(safeFile, []byte("safe"), 0644); err != nil {
		t.Fatalf("failed to create safe file: %v", err)
	}

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "file.txt")
	if err := os.WriteFile(outsideFile, []byte("ESCAPED"), 0644); err != nil {
		t.Fatalf("failed to create outside file: %v", err)
	}

	auth, err := NewPathAuthority(root)
	if err != nil {
		t.Fatalf("failed to create PathAuthority: %v", err)
	}

	// Step 1: ValidatePath passes (point-in-time check)
	_, err = auth.ValidatePath(filepath.Join(root, "data", "file.txt"))
	if err != nil {
		t.Fatalf("expected ValidatePath to succeed: %v", err)
	}

	// Step 2: Simulate TOCTOU attack — swap "data" dir with symlink to outside
	if err := os.RemoveAll(safeDir); err != nil {
		t.Fatalf("failed to remove safe dir: %v", err)
	}
	if err := os.Symlink(outsideDir, safeDir); err != nil {
		t.Fatalf("failed to create malicious symlink: %v", err)
	}

	// Step 3: SafeOpen must REJECT this (kernel-level containment)
	_, err = auth.SafeOpen("data/file.txt")
	if err == nil {
		t.Fatal("TOCTOU race: SafeOpen followed swapped symlink outside the root!")
	}

	// Step 4: Contrast with ValidatePath — it would NOW fail too (post-swap),
	// but the point is that between step 1 and step 2, there's a race window
	// where ValidatePath said "safe" but the path is no longer safe.
	// SafeOpen is atomic and never has this window.
}

func TestSafeStat_KernelContainment(t *testing.T) {
	root := t.TempDir()
	validFile := filepath.Join(root, "stat_target.txt")
	if err := os.WriteFile(validFile, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	auth, err := NewPathAuthority(root)
	if err != nil {
		t.Fatalf("failed to create PathAuthority: %v", err)
	}

	// Valid stat
	info, err := auth.SafeStat("stat_target.txt")
	if err != nil {
		t.Fatalf("expected SafeStat to succeed: %v", err)
	}
	if info.Name() != "stat_target.txt" {
		t.Errorf("expected name stat_target.txt, got %s", info.Name())
	}

	// Reject traversal
	_, err = auth.SafeStat("../../etc/passwd")
	if err == nil || !strings.Contains(err.Error(), "safe stat violation") {
		t.Fatalf("expected SafeStat to reject traversal, got: %v", err)
	}
}



