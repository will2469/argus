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

// N6: Local Allowlist Map — local map literal with compile-time constant strings and no mutations.
func N6_LocalAllowlistMap(ctx context.Context, db DBExecutor, r *http.Request) error {
	localMap := map[string]string{
		"title": "title",
		"price": "price",
	}
	userSort := r.URL.Query().Get("sort")
	col, ok := localMap[userSort]
	if !ok {
		col = "title"
	}

	q := fmt.Sprintf("SELECT id, title FROM products ORDER BY %s ASC", col)
	_, err := db.Query(ctx, q)
	return err
}

// N7: Switch with Return Default — switch where default branch terminates control flow with an error.
func N7_SwitchWithReturnDefault(ctx context.Context, db DBExecutor, userSort string) error {
	var col string
	switch userSort {
	case "name":
		col = "nama"
	case "date":
		col = "created_at"
	default:
		return fmt.Errorf("unsupported sort column: %s", userSort)
	}

	q := fmt.Sprintf("SELECT id FROM users ORDER BY %s DESC", col)
	_, err := db.Query(ctx, q)
	return err
}

// N8: Path-Complete If-Else Direction — both if and else branches assign valid direction literals.
func N8_SafeIfElseDirection(ctx context.Context, db DBExecutor, rawDir string) error {
	var dir string
	if rawDir == "DESC" {
		dir = "DESC"
	} else {
		dir = "ASC"
	}

	q := fmt.Sprintf("SELECT id FROM users ORDER BY created_at %s", dir)
	_, err := db.Query(ctx, q)
	return err
}
