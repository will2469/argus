package negative

import (
	"context"
	"strings"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

func SanitizeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func FormatLikeContains(s string) string {
	return "%" + SanitizeLike(s) + "%"
}

// N1: Obvious Safe — Explicitly sanitized pattern using SanitizeLike.
func N1_ObviousSafe(ctx context.Context, db DB, keyword string) (any, error) {
	safePattern := "%" + SanitizeLike(keyword) + "%"
	const query = "SELECT id, name FROM users WHERE name ILIKE $1 ESCAPE '\\'"
	return db.Query(ctx, query, safePattern)
}

// N2: Legitimate Idiom — Helper function FormatLikeContains.
func N2_LegitimateIdiom(ctx context.Context, db DB, keyword string) (any, error) {
	pattern := FormatLikeContains(keyword)
	const query = "SELECT id, name FROM users WHERE name ILIKE $1"
	return db.Query(ctx, query, pattern)
}

// N3: Unrelated API — Exact equality comparison without LIKE/ILIKE.
func N3_UnrelatedAPI(ctx context.Context, db DB, keyword string) (any, error) {
	const query = "SELECT id, name FROM users WHERE name = $1"
	return db.Query(ctx, query, keyword)
}

// N4: Sanitized Conditional — Both branches sanitize the pattern.
func N4_SanitizedConditional(ctx context.Context, db DB, keyword string, trusted bool) (any, error) {
	pattern := keyword
	if trusted {
		pattern = SanitizeLike(pattern)
	} else {
		pattern = FormatLikeContains(pattern)
	}
	const query = "SELECT id, name FROM users WHERE name ILIKE $1"
	return db.Query(ctx, query, pattern)
}

// N5: Static Constant — Static literal constant passed to LIKE.
func N5_StaticConstant(ctx context.Context, db DB) (any, error) {
	const query = "SELECT id FROM orders WHERE status LIKE $1"
	return db.Query(ctx, query, "PENDING_%")
}
