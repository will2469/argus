package a31_mutation_audit_trail

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/directives"
)

func parseSnippet(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse snippet: %v", err)
	}
	return fset, file
}

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a31/positive",
		"./tests/correctness/a31/negative",
	)
}

func TestA31Direct_ViolationsAndCompliant(t *testing.T) {
	auditMethods := []string{"SaveTx", "RecordTx", "LogAuditEvent", "Save"}
	exemptTables := []string{"sessions", "cache", "temporary_tokens"}

	tests := []struct {
		name        string
		src         string
		expectIssue bool
	}{
		{
			name: "Tx closure with UPDATE users and no audit -> FAIL",
			src: `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	ExecuteTx(ctx context.Context, fn func(tx DB) error) error
}
func Run(ctx context.Context, db DB) {
	db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "UPDATE users SET status = 'ACTIVE' WHERE id = 1")
		return err
	})
}`,
			expectIssue: true,
		},
		{
			name: "Tx closure with UPDATE users and SaveTx audit -> PASS",
			src: `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	ExecuteTx(ctx context.Context, fn func(tx DB) error) error
}
type Auditor interface {
	SaveTx(ctx context.Context, tx DB, event string) error
}
func Run(ctx context.Context, db DB, auditor Auditor) {
	db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "UPDATE users SET status = 'ACTIVE' WHERE id = 1")
		if err != nil { return err }
		return auditor.SaveTx(ctx, tx, "USER_ACTIVATED")
	})
}`,
			expectIssue: false,
		},
		{
			name: "Tx closure mutating exempt sessions table -> PASS",
			src: `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	ExecuteTx(ctx context.Context, fn func(tx DB) error) error
}
func Run(ctx context.Context, db DB) {
	db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "UPDATE sessions SET last_seen = NOW() WHERE id = 1")
		return err
	})
}`,
			expectIssue: false,
		},
		{
			name: "Tx closure with read-only Query -> PASS",
			src: `package main
import "context"
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	ExecuteTx(ctx context.Context, fn func(tx DB) error) error
}
func Run(ctx context.Context, db DB) {
	db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Query(ctx, "SELECT * FROM users WHERE id = 1")
		return err
	})
}`,
			expectIssue: false,
		},
		{
			name: "Tx closure with argus:ignore suppression -> PASS",
			src: `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	ExecuteTx(ctx context.Context, fn func(tx DB) error) error
}
func Run(ctx context.Context, db DB) {
	db.ExecuteTx(ctx, func(tx DB) error {
		// argus:ignore ARGUS-A31 internal diagnostic purge
		_, err := tx.Exec(ctx, "DELETE FROM users WHERE id = 1")
		return err
	})
}`,
			expectIssue: false,
		},
		{
			name: "Interprocedural helper with mutation and no audit -> FAIL",
			src: `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	ExecuteTx(ctx context.Context, fn func(tx DB) error) error
}
func Run(ctx context.Context, db DB) {
	db.ExecuteTx(ctx, func(tx DB) error {
		return helperMutate(ctx, tx)
	})
}
func helperMutate(ctx context.Context, tx DB) error {
	_, err := tx.Exec(ctx, "DELETE FROM orders WHERE id = 1")
	return err
}`,
			expectIssue: true,
		},
		{
			name: "Interprocedural helper recording audit via RecordTx -> PASS",
			src: `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	ExecuteTx(ctx context.Context, fn func(tx DB) error) error
}
type Auditor interface {
	RecordTx(ctx context.Context, event string) error
}
func Run(ctx context.Context, db DB, auditor Auditor) {
	db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "DELETE FROM orders WHERE id = 1")
		if err != nil { return err }
		return recordHelper(ctx, auditor)
	})
}
func recordHelper(ctx context.Context, auditor Auditor) error {
	return auditor.RecordTx(ctx, "ORDER_DELETED")
}`,
			expectIssue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset, file := parseSnippet(t, tt.src)
			dm := directives.ParseGoDirectives(file, fset)
			issues := InspectFile(nil, fset, file, dm, auditMethods, exemptTables)
			if tt.expectIssue {
				if len(issues) == 0 {
					t.Fatalf("expected violation issue, got 0 issues")
				}
				expectedSubstr := "database mutation inside transaction is missing required audit trail logging"
				if !strings.Contains(issues[0].Message, expectedSubstr) {
					t.Fatalf("expected message to contain %q, got %q", expectedSubstr, issues[0].Message)
				}
			} else {
				if len(issues) != 0 {
					t.Fatalf("expected 0 issues, got %d: %+v", len(issues), issues)
				}
			}
		})
	}
}
