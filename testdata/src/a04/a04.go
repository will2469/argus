package a04

import (
	"context"
	"fmt"
	"net/http"
)

var sortAllowlist = map[string]string{
	"name": "nama",
	"date": "created_at",
}

func Sanitize(s string) string {
	return `"` + s + `"`
}

type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func SafeMapLookup(ctx context.Context, db DBExecutor, r *http.Request) error {
	userSort := r.URL.Query().Get("sort")
	safeCol, ok := sortAllowlist[userSort]
	if !ok {
		safeCol = "id"
	}

	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s ASC", safeCol)
	_, err := db.Query(ctx, q)
	return err
}

func SafeSwitchCase(ctx context.Context, db DBExecutor, r *http.Request) error {
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

func BadDirectInput(ctx context.Context, db DBExecutor, r *http.Request) error {
	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s ASC", r.URL.Query().Get("sort")) // want `\[ARGUS-A04\] unsafe dynamic ORDER BY expression`
	_, err := db.Query(ctx, q)
	return err
}

func BadRawVariable(ctx context.Context, db DBExecutor, r *http.Request) error {
	userSort := r.URL.Query().Get("sort")
	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s ASC", userSort) // want `\[ARGUS-A04\] unsafe dynamic ORDER BY variable "userSort"`
	_, err := db.Query(ctx, q)
	return err
}

func BadQuotingSanitizer(ctx context.Context, db DBExecutor, r *http.Request) error {
	userSort := r.URL.Query().Get("sort")
	safeIdent := Sanitize(userSort)
	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s ASC", safeIdent) // want `\[ARGUS-A04\] unsafe dynamic ORDER BY variable "safeIdent"`
	_, err := db.Query(ctx, q)
	return err
}

func IgnoredOrderBy(ctx context.Context, db DBExecutor, r *http.Request) error {
	userSort := r.URL.Query().Get("sort")
	// argus:ignore ARGUS-A04 internal reporting analytics query with trusted admin input
	q := fmt.Sprintf("SELECT id, name FROM users ORDER BY %s ASC", userSort)
	_, err := db.Query(ctx, q)
	return err
}
