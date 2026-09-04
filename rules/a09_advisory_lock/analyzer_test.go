package a09_advisory_lock

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a09/positive",
		"./tests/correctness/a09/negative",
	)
}

func TestInspectAdvisorySQL_Unit(t *testing.T) {
	tests := []struct {
		sql           string
		expectSession bool
		expectIntKey  bool
	}{
		{"SELECT pg_advisory_xact_lock($1)", false, false},
		{"SELECT pg_advisory_xact_lock($1, $2)", false, false},
		{"SELECT pg_advisory_xact_lock(1001, $1)", false, false},
		{"SELECT pg_advisory_xact_lock(namespace_id, resource_id)", false, false},
		{"SELECT pg_advisory_lock($1)", true, false},
		{"SELECT pg_try_advisory_lock($1)", true, false},
		{"SELECT pg_advisory_xact_lock(1)", false, true},
		{"SELECT pg_advisory_xact_lock(1001, 2)", false, true},
		{"SELECT pg_advisory_xact_lock($1, 42)", false, true},
		// AST Determinism: Comments & Literals must NOT trigger false positives
		{"-- comment mentioning pg_advisory_lock\nSELECT 1", false, false},
		{"SELECT 'pg_advisory_lock' AS func_name", false, false},
		{"INVALID SQL SYNTAX pg_advisory_lock", false, false},
	}

	for _, tc := range tests {
		violations := InspectAdvisorySQL(tc.sql)
		hasSession := false
		hasIntKey := false
		for _, v := range violations {
			if v.Type == ViolationSessionLock {
				hasSession = true
			}
			if v.Type == ViolationHardcodedIntKey {
				hasIntKey = true
			}
		}
		if hasSession != tc.expectSession {
			t.Errorf("for query %q, expected session lock=%v, got=%v", tc.sql, tc.expectSession, hasSession)
		}
		if hasIntKey != tc.expectIntKey {
			t.Errorf("for query %q, expected hardcoded key=%v, got=%v", tc.sql, tc.expectIntKey, hasIntKey)
		}
	}
}

func TestNamespaceDelimiters(t *testing.T) {
	// Proves that '.' is rejected as a delimiter while ':' and '/' are accepted.
	validNames := []string{"domain:resource", "orders:123", "payout:lock:123", "domain/resource"}
	for _, name := range validNames {
		if !isStructuredNamespace(name) {
			t.Errorf("expected %q to be a valid structured namespace", name)
		}
	}

	invalidNames := []string{"hello.world", "orders.", ".resource", "foo", "", ":", "/", "hello.world.test"}
	for _, name := range invalidNames {
		if isStructuredNamespace(name) {
			t.Errorf("expected %q to be REJECTED as an unstructured namespace", name)
		}
	}
}

func TestAdvisoryShadowing(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main

type Helper struct{}
func (Helper) WithAdvisoryLock(lockName string, fn func()) {}

var argus Helper

func testShadowViolated() {
	lockName := "orders:user"
	func() {
		lockName := "global"
		argus.WithAdvisoryLock(lockName, func() {})
	}()
	_ = lockName
}

func testShadowSafe() {
	lockName := "global"
	func() {
		lockName := "orders:user"
		argus.WithAdvisoryLock(lockName, func() {})
	}()
	_ = lockName
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		var issues []Issue
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WithAdvisoryLock" {
					if len(call.Args) >= 1 {
						validateLockIdentifier(nil, call.Args[0], fn.Body, &issues)
					}
				}
			}
			return true
		})

		switch fn.Name.Name {
		case "testShadowViolated":
			if len(issues) != 1 {
				t.Fatalf("expected 1 issue for inner shadowed unnamespaced lockName, got %d: %v", len(issues), issues)
			}
		case "testShadowSafe":
			if len(issues) != 0 {
				t.Fatalf("expected 0 issues for inner shadowed valid lockName, got %d: %v", len(issues), issues)
			}
		}
	}
}
