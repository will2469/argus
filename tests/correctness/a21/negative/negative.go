package negative

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

// N1: Obvious Safe — safe queue polling with FOR UPDATE SKIP LOCKED.
func N1_ObviousSafe(ctx context.Context, db DB) {
	const query = `
		SELECT id, payload
		FROM task_queue
		WHERE status = 'PENDING'
		ORDER BY id ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`
	_ = db.QueryRow(ctx, query)
}

// N2: Legitimate Idiom — fail-fast concurrency control with FOR UPDATE NOWAIT.
func N2_LegitimateIdiom(ctx context.Context, db DB, tokenHash string) {
	const query = `
		SELECT id, status
		FROM session_tokens
		WHERE token_hash = $1
		FOR UPDATE NOWAIT
	`
	_ = db.QueryRow(ctx, query, tokenHash)
}

// N3: Point Lookup — single-entity point lookup on primary key column.
func N3_PointLookup(ctx context.Context, db DB, id string) {
	const query = `
		SELECT id, balance
		FROM wallets
		WHERE id = $1
		FOR UPDATE
	`
	_ = db.QueryRow(ctx, query, id)
}

// N4: Normal Select — queries without row locking clauses.
func N4_NormalSelect(ctx context.Context, db DB) {
	const query = `
		SELECT id, name
		FROM users
		WHERE active = true
	`
	_, _ = db.Query(ctx, query)
}

// N5: Aggregation — count query without locks.
func N5_Aggregation(ctx context.Context, db DB) {
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM task_queue WHERE status = 'COMPLETED'")
}
