package security

import (
	"os"
	"testing"
)

func TestValidatePathConfinement(t *testing.T) {
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
			err := ValidatePathConfinement(tc.root, tc.target)
			if tc.expectErr && err == nil {
				t.Fatalf("expected path confinement error, but got nil")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("expected path confinement to pass, got error: %v", err)
			}
		})
	}
}
