package a05_audit_immutability

import (
	"strings"
	"testing"
)

func TestSkeletalTableProvenance(t *testing.T) {
	tables := map[string]bool{
		"audit_logs":      true,
		"security_events": true,
	}

	tests := []struct {
		name        string
		src         string
		expectIssue bool
		msgContains string
	}{
		{
			name: "Dynamic UPDATE accounts with strings.Join -> PASS",
			src: `package main
import "context"
import "strings"
type DB interface { Exec(ctx context.Context, sql string, args ...any) (any, error) }
func Run(ctx context.Context, db DB) {
	setClauses := []string{"name = $1", "status = $2"}
	query := "UPDATE accounts SET " + strings.Join(setClauses, ", ") + " WHERE id = $3"
	db.Exec(ctx, query, "Alice", "ACTIVE", 1)
}`,
			expectIssue: false,
		},
		{
			name: "Dynamic INSERT INTO order_items with strings.Builder -> PASS",
			src: `package main
import "context"
import "strings"
type DB interface { Exec(ctx context.Context, sql string, args ...any) (any, error) }
func Run(ctx context.Context, db DB) {
	var sb strings.Builder
	sb.WriteString("INSERT INTO order_items (order_id, item_code) VALUES ")
	sb.WriteString("($1, $2)")
	db.Exec(ctx, sb.String(), 1, "SKU1")
}`,
			expectIssue: false,
		},
		{
			name: "Dynamic UPDATE audit_logs with strings.Join -> FAIL",
			src: `package main
import "context"
import "strings"
type DB interface { Exec(ctx context.Context, sql string, args ...any) (any, error) }
func Run(ctx context.Context, db DB) {
	setClauses := []string{"action = $1"}
	query := "UPDATE audit_logs SET " + strings.Join(setClauses, ", ") + " WHERE id = $2"
	db.Exec(ctx, query, "HACK", 1)
}`,
			expectIssue: true,
			msgContains: `forbidden UPDATE on audit table "audit_logs"`,
		},
		{
			name: "Dynamic DELETE FROM audit_logs with strings.Builder -> FAIL",
			src: `package main
import "context"
import "strings"
type DB interface { Exec(ctx context.Context, sql string, args ...any) (any, error) }
func Run(ctx context.Context, db DB) {
	var sb strings.Builder
	sb.WriteString("DELETE FROM audit_logs WHERE id = ")
	sb.WriteString("1")
	db.Exec(ctx, sb.String())
}`,
			expectIssue: true,
			msgContains: `forbidden DELETE on audit table "audit_logs"`,
		},
		{
			name: "Pure runtime variable table name -> FAIL (fail-closed unknown provenance)",
			src: `package main
import "context"
import "fmt"
type DB interface { Exec(ctx context.Context, sql string, args ...any) (any, error) }
func Run(ctx context.Context, db DB, table string) {
	query := fmt.Sprintf("UPDATE %s SET status = 1", table)
	db.Exec(ctx, query)
}`,
			expectIssue: true,
			msgContains: "query provenance is unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset, file := parseSnippet(t, tt.src)
			issues := InspectFile(nil, fset, file, nil, tables)
			if tt.expectIssue {
				if len(issues) == 0 {
					t.Fatalf("expected issue containing %q, got 0 issues", tt.msgContains)
				}
				if !strings.Contains(issues[0].Message, tt.msgContains) {
					t.Fatalf("expected issue message containing %q, got %q", tt.msgContains, issues[0].Message)
				}
			} else {
				if len(issues) != 0 {
					t.Fatalf("expected 0 issues, got %d: %+v", len(issues), issues)
				}
			}
		})
	}
}
