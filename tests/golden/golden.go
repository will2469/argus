package golden

import (
	"context"
	"strings"

	"github.com/will2469/argus/tests/golden/pgxpool"
)

var db *pgxpool.Pool

type User struct {
	ID   int
	Name string
}

type Item struct {
	Name string
}

type SearchEngine struct{}

func (SearchEngine) Query(ctx context.Context, q string) string { return q }

func SanitizeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// SHOULD PASS

func SafeQuery(ctx context.Context, pool *pgxpool.Pool, id int) {
	_, err := pool.Query(ctx,
		`SELECT id, name FROM users WHERE id = $1`,
		id,
	)
	_ = err
}

// SHOULD PASS

func Search(ctx context.Context, pool *pgxpool.Pool, input string) {
	pattern := SanitizeLikePattern(input)

	_, err := pool.Query(ctx,
		`SELECT id FROM users WHERE name LIKE $1`,
		pattern,
	)
	_ = err
}

// SHOULD FAIL

func UnsafeSearch(ctx context.Context, pool *pgxpool.Pool, input string) {
	_, err := pool.Query(ctx,
		`SELECT id FROM users WHERE name LIKE $1`,
		input,
	) // want `\[ARGUS-A26\] unsanitized wildcard parameter bound to LIKE/ILIKE clause`
	_ = err
}

// SHOULD FAIL

func UnsafeTenant(ctx context.Context, pool *pgxpool.Pool) {
	_, err := pool.Query(ctx,
		`SELECT id FROM users WHERE tenant_id IS NOT NULL`,
	) // want `\[ARGUS-A24\] query on multi-tenant table 'users' missing 'tenant_id' predicate`
	_ = err
}

// SHOULD NOT FLAG A17

func UnrelatedQuery(ctx context.Context, search SearchEngine, items []Item) {
	for _, item := range items {
		search.Query(ctx, item.Name)
	}
}

// SHOULD FAIL A17

func NPlusOne(ctx context.Context, pool *pgxpool.Pool, users []User) {
	for _, user := range users { // want `\[ARGUS-A17\] N\+1 query pattern detected`
		pool.QueryRow(ctx,
			`SELECT id FROM profiles WHERE user_id = $1`,
			user.ID,
		)
	}
}

// SHOULD FAIL A17

func NPlusOneDeep(ctx context.Context, users []User) {
	for _, user := range users { // want `\[ARGUS-A17\] N\+1 query pattern detected`
		loadProfile(ctx, user.ID)
	}
}

func loadProfile(ctx context.Context, id int) {
	fetchProfile(ctx, id)
}

func fetchProfile(ctx context.Context, id int) {
	db.Query(ctx, "SELECT id, name FROM profiles WHERE id = $1", id)
}
