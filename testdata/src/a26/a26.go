package a26

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

// 1. Safe query using SanitizeLike (Compliant)
func SafeSanitizedSearch(ctx context.Context, db DB, keyword string) (any, error) {
	safePattern := "%" + SanitizeLike(keyword) + "%"
	const query = "SELECT id, name FROM users WHERE name ILIKE $1 ESCAPE '\\'"
	return db.Query(ctx, query, safePattern)
}

// 2. Safe query using FormatLikeContains (Compliant)
func SafeHelperSearch(ctx context.Context, db DB, keyword string) (any, error) {
	pattern := FormatLikeContains(keyword)
	const query = "SELECT id, name FROM users WHERE name ILIKE $1"
	return db.Query(ctx, query, pattern)
}

// 3. Safe constant LIKE pattern (Compliant)
func SafeConstantLike(ctx context.Context, db DB) (any, error) {
	const query = "SELECT id FROM orders WHERE status LIKE $1"
	return db.Query(ctx, query, "PENDING_%")
}

// 4. Unsafe raw keyword bound to ILIKE (Violation)
func UnsafeRawKeyword(ctx context.Context, db DB, keyword string) (any, error) {
	const query = "SELECT id, name FROM users WHERE name ILIKE $1"
	return db.Query(ctx, query, keyword) // want `\[ARGUS-A26\] unsanitized wildcard parameter bound to LIKE/ILIKE clause \(\$1\); risk of pattern language hijacking, PII exposure, and sequential scan DoS \(CWE-89, CWE-400\)`
}

// 5. Unsafe raw concatenation bound to ILIKE (Violation)
func UnsafeRawConcat(ctx context.Context, db DB, keyword string) (any, error) {
	pattern := "%" + keyword + "%"
	const query = "SELECT id, name FROM users WHERE name ILIKE $1"
	return db.Query(ctx, query, pattern) // want `\[ARGUS-A26\] unsanitized wildcard parameter bound to LIKE/ILIKE clause \(\$1\); risk of pattern language hijacking, PII exposure, and sequential scan DoS \(CWE-89, CWE-400\)`
}

// 6. Ignored via directive
func IgnoredSearch(ctx context.Context, db DB, rawPattern string) (any, error) {
	const query = "SELECT id FROM logs WHERE msg LIKE $1"
	// argus:ignore ARGUS-A26 internal admin regex query
	return db.Query(ctx, query, rawPattern)
}

// 7. Ignored via shortcode
func IgnoredShortcode(ctx context.Context, db DB, rawPattern string) (any, error) {
	const query = "SELECT id FROM logs WHERE msg LIKE $1"
	// argus:ignore-a26 internal diagnostic query
	return db.Query(ctx, query, rawPattern)
}
