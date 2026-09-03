package positive

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

// P1: Obvious Violation — Unsafe raw keyword bound to ILIKE clause.
func P1_Obvious(ctx context.Context, db DB, keyword string) (any, error) {
	const query = "SELECT id, name FROM users WHERE name ILIKE $1"
	return db.Query(ctx, query, keyword) // want `\[ARGUS-A26\] unsanitized wildcard parameter bound to LIKE/ILIKE clause \(\$1\); risk of pattern language hijacking, PII exposure, and sequential scan DoS \(CWE-89, CWE-400\)`
}

// P2: Indirect Violation — Unsafe raw concatenation bound to ILIKE.
func P2_Indirect(ctx context.Context, db DB, keyword string) (any, error) {
	pattern := "%" + keyword + "%"
	const query = "SELECT id, name FROM users WHERE name ILIKE $1"
	return db.Query(ctx, query, pattern) // want `\[ARGUS-A26\] unsanitized wildcard parameter bound to LIKE/ILIKE clause \(\$1\); risk of pattern language hijacking, PII exposure, and sequential scan DoS \(CWE-89, CWE-400\)`
}

// P3: Helper Violation — Unrelated ReplaceAll that does not escape wildcards.
func P3_Helper(ctx context.Context, db DB, keyword string) (any, error) {
	safe := strings.ReplaceAll(keyword, "foo", "bar")
	const query = "SELECT id, name FROM users WHERE name ILIKE $1"
	return db.Query(ctx, query, safe) // want `\[ARGUS-A26\] unsanitized wildcard parameter bound to LIKE/ILIKE clause \(\$1\); risk of pattern language hijacking, PII exposure, and sequential scan DoS \(CWE-89, CWE-400\)`
}

// P4: Nested Violation — Sequential overwrite: sanitized then reassigned to raw.
func P4_Nested(ctx context.Context, db DB, keyword string) (any, error) {
	pattern := keyword
	pattern = SanitizeLike(pattern)
	pattern = keyword
	const query = "SELECT id, name FROM users WHERE name ILIKE $1"
	return db.Query(ctx, query, pattern) // want `\[ARGUS-A26\] unsanitized wildcard parameter bound to LIKE/ILIKE clause \(\$1\); risk of pattern language hijacking, PII exposure, and sequential scan DoS \(CWE-89, CWE-400\)`
}

// P5: Alias Violation — Conditional without else branch: only sanitized on one path.
func P5_Alias(ctx context.Context, db DB, keyword string, trusted bool) (any, error) {
	pattern := keyword
	if trusted {
		pattern = SanitizeLike(pattern)
	}
	const query = "SELECT id, name FROM users WHERE name ILIKE $1"
	return db.Query(ctx, query, pattern) // want `\[ARGUS-A26\] unsanitized wildcard parameter bound to LIKE/ILIKE clause \(\$1\); risk of pattern language hijacking, PII exposure, and sequential scan DoS \(CWE-89, CWE-400\)`
}

// P_Ignored: Suppressed violation using canonical shortcode.
func P_Ignored(ctx context.Context, db DB, rawPattern string) (any, error) {
	const query = "SELECT id FROM logs WHERE msg LIKE $1"
	// argus:ignore-a26 internal diagnostic query
	return db.Query(ctx, query, rawPattern)
}
