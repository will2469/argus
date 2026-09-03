package negative

import (
	"context"
	"fmt"
	"net/http"
)

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

var sortAllowlist = map[string]string{
	"name": "nama",
	"date": "created_at",
}

// N1: Obvious Safe — closed-set allowlist map lookup.
func N1_ObviousSafe(ctx context.Context, db DBExecutor, r *http.Request) error {
	userSort := r.URL.Query().Get("sort")
	safeCol, ok := sortAllowlist[userSort]
	if !ok {
		safeCol = "id"
	}

	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s ASC", safeCol)
	_, err := db.Query(ctx, q)
	return err
}

// N2: Legitimate Idiom — switch-case mapping to static string literals.
func N2_LegitimateIdiom(ctx context.Context, db DBExecutor, r *http.Request) error {
	userSort := r.URL.Query().Get("sort")
	var safeCol string
	switch userSort {
	case "name":
		safeCol = "nama"
	case "date":
		safeCol = "created_at"
	default:
		safeCol = "id"
	}

	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s DESC", safeCol)
	_, err := db.Query(ctx, q)
	return err
}

// N3: Unrelated API — non-SQL formatting using phrase "order by".
func N3_UnrelatedAPI(userInput string) string {
	return fmt.Sprintf("Customer order by priority status: %s", userInput)
}

// N4: Safe Direction Validation — direction strictly verified against ASC/DESC.
func N4_SafeDirection(ctx context.Context, db DBExecutor, rawDir string) error {
	dir := "ASC"
	if rawDir == "DESC" {
		dir = "DESC"
	}

	q := fmt.Sprintf("SELECT id FROM users ORDER BY id %s", dir)
	_, err := db.Query(ctx, q)
	return err
}

// N5: Static Constant Input — compile-time static string literal passed to ORDER BY.
func N5_StaticConstant(ctx context.Context, db DBExecutor) error {
	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s", "created_at DESC")
	_, err := db.Query(ctx, q)
	return err
}
