package a17

import (
	"context"
)

type DB struct{}

func (DB) Query(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

func (DB) QueryRow(ctx context.Context, sql string, args ...any) any {
	return nil
}

func (DB) Exec(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

type NonDBWorker struct{}

func (NonDBWorker) Exec(cmd string) error {
	return nil
}

func helperQuery(ctx context.Context, db DB, id int) (string, error) {
	_ = db.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", id)
	return "", nil
}

func Cases(ctx context.Context, db DB, worker NonDBWorker, ids []int) {
	// 1. QueryRow inside range loop
	for _, id := range ids { // want `\[ARGUS-A17\] N\+1 query pattern detected \(loop \+ Query\); use set-based query \(ANY\(\$1\)\) or batch operations instead of querying inside loop`
		_ = db.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", id)
	}

	// 2. Exec inside for loop
	for i := 0; i < len(ids); i++ { // want `\[ARGUS-A17\] N\+1 query pattern detected \(loop \+ Query\); use set-based query \(ANY\(\$1\)\) or batch operations instead of querying inside loop`
		_, _ = db.Exec(ctx, "UPDATE users SET active = true WHERE id = $1", ids[i])
	}

	// 3. Nested loop with query inside inner loop
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ { // want `\[ARGUS-A17\] nested N\+1 query pattern detected \(loop depth 2 \+ Query\); use set-based query \(ANY\(\$1\)\) or batch operations instead of querying inside loop`
			_ = db.QueryRow(ctx, "SELECT 1 FROM dual")
		}
	}

	// 4. Non-DB worker calling Exec inside loop (compliant - type check verified!)
	for range ids {
		_ = worker.Exec("echo hello")
	}

	// 5. Helper executing DB query called inside loop
	for _, id := range ids { // want `\[ARGUS-A17\] N\+1 query pattern detected: helper function "helperQuery" executes database query inside loop; use batching or set-based query instead`
		_, _ = helperQuery(ctx, db, id)
	}

	// 6. Compliant set-based query outside loop
	_, _ = db.Query(ctx, "SELECT email FROM users WHERE id = ANY($1)", ids)

	// 7. Compliant retry loop over integer constant
	for range 3 {
		_ = db.QueryRow(ctx, "SELECT 1")
	}

	// 8. Ignored loop via canonical shortcode
	// argus:ignore-a17 keyset pagination batch processor
	for _, id := range ids {
		_ = db.QueryRow(ctx, "SELECT * FROM users WHERE id = $1", id)
	}
}

func getUser(ctx context.Context, db DB, id int) (string, error) {
	_ = db.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", id)
	return "", nil
}

func loadUser(ctx context.Context, db DB, id int) (string, error) {
	return getUser(ctx, db, id)
}

func fetchUserWrapper(ctx context.Context, db DB, id int) (string, error) {
	return loadUser(ctx, db, id)
}

func UnsafeMultiLevelHelperLoop(ctx context.Context, db DB, ids []int) {
	// 9. Multi-level helper executing DB query called inside loop (Depth 3)
	for _, id := range ids { // want `\[ARGUS-A17\] N\+1 query pattern detected: helper function "fetchUserWrapper" executes database query inside loop; use batching or set-based query instead`
		_, _ = fetchUserWrapper(ctx, db, id)
	}
}

type SearchEngine struct{}

func (SearchEngine) Query(ctx context.Context, q string) string {
	return q
}

func SafeSearchEngineLoop(ctx context.Context, search SearchEngine, items []string) {
	// 10. Non-DB SearchEngine.Query called inside loop (Compliant - zero findings!)
	for _, item := range items {
		_ = search.Query(ctx, item)
	}
}
