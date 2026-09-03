package security

import (
	"os"
	"testing"
)

func TestCheckPathConfinement(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	tests := []struct {
		name      string
		root      string
		target    string
		expectErr bool
	}{
		{
			name:      "current directory relative",
			root:      ".",
			target:    ".",
			expectErr: false,
		},
		{
			name:      "child directory relative",
			root:      ".",
			target:    "rules",
			expectErr: false,
		},
		{
			name:      "nested child directory relative",
			root:      ".",
			target:    "rules/a01_tenant_isolation",
			expectErr: false,
		},
		{
			name:      "absolute path within root",
			root:      cwd,
			target:    cwd + "/rules",
			expectErr: false,
		},
		{
			name:      "path traversal outside root relative",
			root:      ".",
			target:    "../../anything",
			expectErr: true,
		},
		{
			name:      "path traversal hidden in middle",
			root:      ".",
			target:    "rules/../../..",
			expectErr: true,
		},
		{
			name:      "absolute path to root filesystem",
			root:      cwd,
			target:    "/etc/passwd",
			expectErr: true,
		},
		{
			name:      "empty path",
			root:      ".",
			target:    "",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckPathConfinement(tc.root, tc.target)
			if tc.expectErr && err == nil {
				t.Fatalf("expected path confinement error, but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("expected path confinement to pass, got error: %v", err)
			}
		})
	}
}

func TestSafeOpenDir(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	// Create a test file in current directory
	testFile := "test_safe_open.txt"
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Test SafeOpenDir with valid file
	f, err := SafeOpenDir(cwd, testFile)
	if err != nil {
		t.Fatalf("SafeOpenDir failed for valid file: %v", err)
	}
	defer f.Close()

	// Test SafeOpenDir with path traversal
	_, err = SafeOpenDir(cwd, "../../etc/passwd")
	if err == nil {
		t.Fatal("SafeOpenDir should reject path traversal")
	}

	// Test SafeOpenDir with absolute path
	_, err = SafeOpenDir(cwd, "/etc/passwd")
	if err == nil {
		t.Fatal("SafeOpenDir should reject absolute path outside root")
	}
}

func TestSafeStatDir(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	// Create a test file in current directory
	testFile := "test_safe_stat.txt"
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Test SafeStatDir with valid file
	info, err := SafeStatDir(cwd, testFile)
	if err != nil {
		t.Fatalf("SafeStatDir failed for valid file: %v", err)
	}
	if info == nil {
		t.Fatal("SafeStatDir returned nil FileInfo for valid file")
	}

	// Test SafeStatDir with path traversal
	_, err = SafeStatDir(cwd, "../../etc/passwd")
	if err == nil {
		t.Fatal("SafeStatDir should reject path traversal")
	}

	// Test SafeStatDir with absolute path
	_, err = SafeStatDir(cwd, "/etc/passwd")
	if err == nil {
		t.Fatal("SafeStatDir should reject absolute path outside root")
	}
}
