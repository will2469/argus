package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScanTool_CallSiteParity_TOCTOURaceDefense asserts that even if an attacker
// swaps a validated directory with a symlink pointing outside the allowed root
// before Execute runs, kernel-level capability containment prevents reading
// or leaking sensitive files outside the boundary.
func TestScanTool_CallSiteParity_TOCTOURaceDefense(t *testing.T) {
	projectRoot := t.TempDir()
	srcDir := filepath.Join(projectRoot, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	validFile := filepath.Join(srcDir, "valid.go")
	if err := os.WriteFile(validFile, []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("failed to write valid.go: %v", err)
	}

	// Setup sensitive directory outside allowed root
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.go")
	sensitivePayload := "package sensitive\n// SENSITIVE_TOKEN_LEAK_SECRET_KEY\nfunc Secret() { _ = \"FORBIDDEN\" }\n"
	if err := os.WriteFile(outsideFile, []byte(sensitivePayload), 0644); err != nil {
		t.Fatalf("failed to write secret.go: %v", err)
	}

	tool := NewScanToolWithRoots([]string{projectRoot})
	req := json.RawMessage(`{"dirs": ["src"]}`)

	// STEP 1: Point-in-time policy validation succeeds for valid directory
	if err := tool.ValidatePolicy(req); err != nil {
		t.Fatalf("ValidatePolicy expected to succeed on initial valid directory, got: %v", err)
	}

	// STEP 2: TOCTOU race simulation — swap "src" with a symlink to outsideDir
	if err := os.RemoveAll(srcDir); err != nil {
		t.Fatalf("failed to remove srcDir: %v", err)
	}
	if err := os.Symlink(outsideDir, srcDir); err != nil {
		t.Fatalf("failed to create swapped symlink: %v", err)
	}

	// STEP 3: Execute tool
	ctx := context.Background()
	resp := tool.Execute(ctx, 1, req)

	// STEP 4: Call-site parity assertions
	respBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}
	respStr := string(respBytes)

	t.Logf("Response under TOCTOU attack: %s", respStr)

	// Containment assertion: Under NO circumstance should sensitive payloads be leaked
	if strings.Contains(respStr, "SENSITIVE_TOKEN_LEAK_SECRET_KEY") || strings.Contains(respStr, "FORBIDDEN") {
		t.Fatalf("CRITICAL SECURITY VULNERABILITY: tool.Execute leaked outside contents!\nResponse: %s", respStr)
	}
}

func TestScanTool_CallSiteParity_ValidScanning(t *testing.T) {
	projectRoot := t.TempDir()
	srcDir := filepath.Join(projectRoot, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	code := "package main\n\nfunc Run() {\n\t// Compliant Go code\n}\n"
	if err := os.WriteFile(filepath.Join(srcDir, "app.go"), []byte(code), 0644); err != nil {
		t.Fatalf("failed to write app.go: %v", err)
	}

	tool := NewScanToolWithRoots([]string{projectRoot})
	req := json.RawMessage(`{"dirs": ["src"]}`)

	if err := tool.ValidatePolicy(req); err != nil {
		t.Fatalf("expected ValidatePolicy to succeed, got: %v", err)
	}

	resp := tool.Execute(context.Background(), 2, req)
	respBytes, _ := json.Marshal(resp)
	respStr := string(respBytes)

	if !strings.Contains(respStr, "CLEAN") {
		t.Fatalf("expected clean scan result, got: %s", respStr)
	}
}

func TestScanTool_CallSiteParity_MigrationTOCTOUDefense(t *testing.T) {
	projectRoot := t.TempDir()
	migDir := filepath.Join(projectRoot, "migrations")
	if err := os.Mkdir(migDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(migDir, "001_init.up.sql"), []byte("CREATE TABLE foo (id serial);"), 0644)
	_ = os.WriteFile(filepath.Join(migDir, "001_init.down.sql"), []byte("DROP TABLE foo;"), 0644)

	outsideDir := t.TempDir()
	outsideSQL := filepath.Join(outsideDir, "002_secret.up.sql")
	_ = os.WriteFile(outsideSQL, []byte("CREATE TABLE secret_db_leak (secret_col text);"), 0644)

	tool := NewScanToolWithRoots([]string{projectRoot})
	req := json.RawMessage(`{"migrations": ["migrations"]}`)

	if err := tool.ValidatePolicy(req); err != nil {
		t.Fatal(err)
	}

	// TOCTOU: swap migrations dir with outside symlink
	if err := os.RemoveAll(migDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, migDir); err != nil {
		t.Fatal(err)
	}

	resp := tool.Execute(context.Background(), 3, req)
	respBytes, _ := json.Marshal(resp)
	respStr := string(respBytes)

	if strings.Contains(respStr, "secret_col") || strings.Contains(respStr, "secret_db_leak") {
		t.Fatalf("CRITICAL SECURITY ESCAPE: Migration runner leaked outside SQL!\nResp: %s", respStr)
	}
}
