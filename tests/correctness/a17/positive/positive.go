package positive

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

func helperQuery(ctx context.Context, db DB, id int) (string, error) {
	_ = db.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", id)
	return "", nil
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

// P1: Obvious Violation — QueryRow inside range loop.
func P1_Obvious(ctx context.Context, db DB, ids []int) {
	for _, id := range ids { // want `\[ARGUS-A17\] N\+1 query pattern detected \(loop \+ Query\); use set-based query \(ANY\(\$1\)\) or batch operations instead of querying inside loop`
		_ = db.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", id)
	}
}

// P2: Indirect Violation — Exec inside classic for loop.
func P2_Indirect(ctx context.Context, db DB, ids []int) {
	for i := 0; i < len(ids); i++ { // want `\[ARGUS-A17\] N\+1 query pattern detected \(loop \+ Query\); use set-based query \(ANY\(\$1\)\) or batch operations instead of querying inside loop`
		_, _ = db.Exec(ctx, "UPDATE users SET active = true WHERE id = $1", ids[i])
	}
}

// P3: Helper Violation — Helper function executing DB query called inside loop.
func P3_Helper(ctx context.Context, db DB, ids []int) {
	for _, id := range ids { // want `\[ARGUS-A17\] N\+1 query pattern detected: helper function "helperQuery" executes database query inside loop; use batching or set-based query instead`
		_, _ = helperQuery(ctx, db, id)
	}
}

// P4: Nested Violation — Query inside nested loop (depth 2).
func P4_Nested(ctx context.Context, db DB) {
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ { // want `\[ARGUS-A17\] nested N\+1 query pattern detected \(loop depth 2 \+ Query\); use set-based query \(ANY\(\$1\)\) or batch operations instead of querying inside loop`
			_ = db.QueryRow(ctx, "SELECT 1 FROM dual")
		}
	}
}

// P5: Alias Violation — Multi-level transitive helper called inside loop.
func P5_Alias(ctx context.Context, db DB, ids []int) {
	for _, id := range ids { // want `\[ARGUS-A17\] N\+1 query pattern detected: helper function "fetchUserWrapper" executes database query inside loop; use batching or set-based query instead`
		_, _ = fetchUserWrapper(ctx, db, id)
	}
}

func hydrateUser(ctx context.Context, db DB, id int) (string, error) {
	_ = db.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", id)
	return "", nil
}

type UserRepo struct{}

func (UserRepo) Get(ctx context.Context, db DB, id int) {
	_ = db.QueryRow(ctx, "SELECT 1 WHERE id = $1", id)
}

// P6: Semantic Helper Violation — Function executing DB query called inside loop without lexical prefix (hydrateUser).
func P6_SemanticHelper(ctx context.Context, db DB, ids []int) {
	for _, id := range ids { // want `\[ARGUS-A17\] N\+1 query pattern detected: helper function "hydrateUser" executes database query inside loop; use batching or set-based query instead`
		_, _ = hydrateUser(ctx, db, id)
	}
}

// P7: Receiver Method Helper Violation — Receiver method executing DB query called inside loop.
func P7_ReceiverMethodHelper(ctx context.Context, db DB, repo UserRepo, ids []int) {
	for _, id := range ids { // want `\[ARGUS-A17\] N\+1 query pattern detected: helper function "\(UserRepo\).Get" executes database query inside loop; use batching or set-based query instead`
		repo.Get(ctx, db, id)
	}
}

// P_Ignored: Suppressed violation using canonical shortcode.
func P_Ignored(ctx context.Context, db DB, ids []int) {
	// argus:ignore-a17 keyset pagination batch processor
	for _, id := range ids {
		_ = db.QueryRow(ctx, "SELECT * FROM users WHERE id = $1", id)
	}
}
