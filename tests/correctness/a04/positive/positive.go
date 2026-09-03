package positive

import (
	"context"
	"fmt"
	"net/http"
)

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func Sanitize(s string) string {
	return `"` + s + `"`
}

// P1: Obvious Violation — direct raw user expression passed to ORDER BY format string.
func P1_Obvious(ctx context.Context, db DBExecutor, r *http.Request) error {
	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s ASC", r.URL.Query().Get("sort")) // want `\[ARGUS-A04\] unsafe dynamic ORDER BY expression`
	_, err := db.Query(ctx, q)
	return err
}

// P2: Indirect Violation — untrusted variable passed into dynamic ORDER BY without allowlist mapping.
func P2_Indirect(ctx context.Context, db DBExecutor, r *http.Request) error {
	userSort := r.URL.Query().Get("sort")
	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s ASC", userSort) // want `\[ARGUS-A04\] unsafe dynamic ORDER BY variable "userSort"`
	_, err := db.Query(ctx, q)
	return err
}

// P3: Helper Violation — unsafe dynamic ORDER BY constructed inside internal helper subroutine.
func P3_Helper(ctx context.Context, db DBExecutor, sort string) error {
	q := buildOrderQueryHelper(sort)
	_, err := db.Query(ctx, q)
	return err
}

func buildOrderQueryHelper(sortCol string) string {
	return fmt.Sprintf("SELECT id FROM profiles ORDER BY %s DESC", sortCol) // want `\[ARGUS-A04\] unsafe dynamic ORDER BY variable "sortCol"`
}

// P4: Nested Violation — dynamic ORDER BY constructed inside loop and conditional block.
func P4_Nested(ctx context.Context, db DBExecutor, sorts []string) error {
	for _, s := range sorts {
		if s != "" {
			q := fmt.Sprintf("SELECT id FROM audit_logs ORDER BY %s ASC", s) // want `\[ARGUS-A04\] unsafe dynamic ORDER BY variable "s"`
			if _, err := db.Query(ctx, q); err != nil {
				return err
			}
		}
	}
	return nil
}

// P5: Alias Violation — quoting sanitizer used as pseudo-protection instead of closed-set allowlist.
func P5_Alias(ctx context.Context, db DBExecutor, r *http.Request) error {
	userSort := r.URL.Query().Get("sort")
	safeIdent := Sanitize(userSort)
	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s ASC", safeIdent) // want `\[ARGUS-A04\] unsafe dynamic ORDER BY variable "safeIdent"`
	_, err := db.Query(ctx, q)
	return err
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, db DBExecutor, r *http.Request) error {
	userSort := r.URL.Query().Get("sort")
	// argus:ignore ARGUS-A04 internal analytics query with trusted admin column input
	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s ASC", userSort)
	_, err := db.Query(ctx, q)
	return err
}
