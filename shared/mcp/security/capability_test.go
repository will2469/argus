package security

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathAuthority_AuthorizeAndOpen(t *testing.T) {
	root := t.TempDir()
	subDir := filepath.Join(root, "src")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(subDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}

	auth, err := NewPathAuthority(root)
	if err != nil {
		t.Fatalf("failed to create authority: %v", err)
	}

	// 1. Valid directories
	cap, cleanDirs, cleanMigs, err := auth.AuthorizeAndOpen([]string{"src"}, nil)
	if err != nil {
		t.Fatalf("expected AuthorizeAndOpen to succeed, got: %v", err)
	}
	defer cap.Close()

	if len(cleanDirs) != 1 || cleanDirs[0] != "src" {
		t.Fatalf("expected cleanDirs [src], got: %v", cleanDirs)
	}
	if len(cleanMigs) != 0 {
		t.Fatalf("expected cleanMigs empty, got: %v", cleanMigs)
	}

	// Read file via capability FS
	content, err := fs.ReadFile(cap.FS(), "src/main.go")
	if err != nil {
		t.Fatalf("failed to read file via cap.FS(): %v", err)
	}
	if string(content) != "package main" {
		t.Fatalf("expected 'package main', got %s", string(content))
	}

	// 2. Traversal attempt
	_, _, _, err = auth.AuthorizeAndOpen([]string{"../../etc"}, nil)
	if err == nil || !strings.Contains(err.Error(), "path authority violation") {
		t.Fatalf("expected traversal to be rejected, got: %v", err)
	}
}
