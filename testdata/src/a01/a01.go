package a01

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	QueryRow(ctx context.Context, sql string, args ...any) any
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

type UserRequest struct {
	ID string
}

func SanitizeIdentifier(ident string) string {
	return `"` + ident + `"`
}

func SafeQuery(ctx context.Context, db DBExecutor, req UserRequest) {
	db.Query(ctx, "SELECT id, name FROM users WHERE id = $1", req.ID)
}

func SafeConcatLiterals(ctx context.Context, db DBExecutor) {
	db.Query(ctx, "SELECT id, name "+"FROM users WHERE active = true")
}

func SafeSanitized(ctx context.Context, db DBExecutor, table string) {
	sql := "SELECT * FROM " + SanitizeIdentifier(table)
	db.Query(ctx, sql)
}

func SafeParameterizedBuilder(ctx context.Context, db DBExecutor) {
	var sb strings.Builder
	sb.WriteString("SELECT id, name FROM users WHERE active = true")
	sb.WriteString(" AND role = $1")
	db.Query(ctx, sb.String(), "admin")
}

func SafeSprintfPlaceholder(ctx context.Context, db DBExecutor, id string) {
	idx := 1
	query := "SELECT id, name FROM users WHERE 1=1"
	query += fmt.Sprintf(" AND id = $%d", idx)
	db.Query(ctx, query, id)
}

func BadSQLSprintf(ctx context.Context, db DBExecutor, req UserRequest) {
	sql := fmt.Sprintf("SELECT id, name FROM users WHERE id = '%s'", req.ID)
	db.Query(ctx, sql) // want `\[ARGUS-A01\] unsafe SQL concatenation or formatting`
}

func BadSQLConcat(ctx context.Context, db DBExecutor, r *http.Request) {
	id := r.URL.Query().Get("id")
	sql := "SELECT id, name FROM users WHERE id = '" + id + "'"
	db.QueryRow(ctx, sql) // want `\[ARGUS-A01\] unsafe SQL concatenation or formatting`
}

func BadInlineConcat(ctx context.Context, db DBExecutor, id string) {
	db.Exec(ctx, "DELETE FROM users WHERE id = "+id) // want `\[ARGUS-A01\] unsafe SQL concatenation or formatting`
}

func BadBuilderRawInput(ctx context.Context, db DBExecutor, req UserRequest) {
	var sb strings.Builder
	sb.WriteString("SELECT id FROM users WHERE status = '")
	sb.WriteString(req.ID)
	sb.WriteString("'")
	db.Query(ctx, sb.String()) // want `\[ARGUS-A01\] unsafe SQL concatenation or formatting`
}

func IgnoredBadSQL(ctx context.Context, db DBExecutor, id string) {
	// argus:ignore ARGUS-A01 legacy maintenance script with verified numeric id
	db.Exec(ctx, "DELETE FROM temp_sessions WHERE id = "+id)
}

func IgnoredBadBuilder(ctx context.Context, db DBExecutor, req UserRequest) {
	var sb strings.Builder
	sb.WriteString("SELECT id FROM users WHERE status = '")
	sb.WriteString(req.ID)
	sb.WriteString("'")
	// argus:ignore ARGUS-A01 isolated admin query builder
	db.Query(ctx, sb.String())
}
