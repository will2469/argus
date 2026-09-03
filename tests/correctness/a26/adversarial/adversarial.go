package adversarial

import (
	"context"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

// A1: Branch — conditional unsanitized LIKE query.
func A1_Branch(ctx context.Context, db DB, input string, cond bool) {
	if cond {
		_, _ = db.Query(ctx, "SELECT id FROM users WHERE email LIKE $1", input)
	}
}

// A2: Reassignment — string variable mutated with prefix concatenation.
func A2_Reassignment(ctx context.Context, db DB, input string) {
	p := input
	p = "%" + p
	_, _ = db.Query(ctx, "SELECT id FROM items WHERE title ILIKE $1", p)
}

// A3: Alias — parameter passed via variable alias.
func A3_Alias(ctx context.Context, db DB, input string) {
	aliased := input
	_, _ = db.Query(ctx, "SELECT id FROM products WHERE sku LIKE $1", aliased)
}

// A4: Wrapper — DAO struct method executing unsanitized LIKE query.
type UserSearcher struct {
	db DB
}

func (s UserSearcher) Search(ctx context.Context, name string) {
	_, _ = s.db.Query(ctx, "SELECT id FROM users WHERE name ILIKE $1", name)
}

// A5: Nested Function — closure executing unsanitized LIKE query.
func A5_NestedFunction(ctx context.Context, db DB, input string) {
	searchFn := func() {
		_, _ = db.Query(ctx, "SELECT id FROM docs WHERE title ILIKE $1", input)
	}
	searchFn()
}

// A6: Generic — generic finder executing unsanitized LIKE query.
type TagFinder[T any] struct {
	db DB
}

func (f TagFinder[T]) Find(ctx context.Context, tag string) {
	_, _ = f.db.Query(ctx, "SELECT id FROM tags WHERE label LIKE $1", tag)
}

// A7: MultiParam — LIKE wildcard at second parameter position ($2).
func A7_MultiParam(ctx context.Context, db DB, orgID, rawMsg string) {
	_, _ = db.Query(ctx, "SELECT id FROM logs WHERE org_id = $1 AND msg ILIKE $2", orgID, rawMsg)
}
