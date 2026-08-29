package a21

import (
	"context"
)

type DB struct{}

func (DB) Exec(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

func (DB) Query(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

func (DB) QueryRow(ctx context.Context, sql string, args ...any) any {
	return nil
}

// 1. Safe queue polling with FOR UPDATE SKIP LOCKED (Compliant)
func SafeSkipLocked(ctx context.Context, db DB) {
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

// 2. Safe fail-fast locking with FOR UPDATE NOWAIT (Compliant)
func SafeNoWait(ctx context.Context, db DB, tokenHash string) {
	const query = `
		SELECT id, status
		FROM session_tokens
		WHERE token_hash = $1
		FOR UPDATE NOWAIT
	`
	_ = db.QueryRow(ctx, query, tokenHash)
}

// 3. Safe single-entity point lookup on primary key (Compliant)
func SafePointLookup(ctx context.Context, db DB, id string) {
	const query = `
		SELECT id, balance
		FROM wallets
		WHERE id = $1
		FOR UPDATE
	`
	_ = db.QueryRow(ctx, query, id)
}

// 4. Unsafe queue polling with blocking FOR UPDATE (Violation)
func UnsafeBlockingQueue(ctx context.Context, db DB) {
	const query = `
		SELECT id, payload
		FROM task_queue
		WHERE status = 'PENDING'
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE
	`
	_ = db.QueryRow(ctx, query) // want `\[ARGUS-A21\] blocking row-level lock \(FOR UPDATE / FOR NO KEY UPDATE\) without SKIP LOCKED or NOWAIT; risk of worker lock convoy \(CWE-662, CWE-833\)`
}

// 5. Unsafe multi-row status scan with blocking FOR NO KEY UPDATE (Violation)
func UnsafeNoKeyUpdate(ctx context.Context, db DB, tenantID string) {
	const query = `
		SELECT id, amount
		FROM pending_payments
		WHERE tenant_id = $1
		LIMIT 10
		FOR NO KEY UPDATE
	`
	_, _ = db.Query(ctx, query, tenantID) // want `\[ARGUS-A21\] blocking row-level lock \(FOR UPDATE / FOR NO KEY UPDATE\) without SKIP LOCKED or NOWAIT; risk of worker lock convoy \(CWE-662, CWE-833\)`
}

// 6. Normal SELECT without locks (Compliant)
func SafeSelectNormal(ctx context.Context, db DB) {
	const query = `
		SELECT id, name
		FROM users
		WHERE active = true
	`
	_, _ = db.Query(ctx, query)
}

// 7. Ignored blocking lock via directive
func IgnoredBlockingLock(ctx context.Context, db DB) {
	const query = `
		SELECT id, payload
		FROM task_queue
		WHERE status = 'PENDING'
		FOR UPDATE
	`
	// argus:ignore ARGUS-A21 offline single worker exclusive
	_ = db.QueryRow(ctx, query)
}

// 8. Ignored blocking lock via shortcode
func IgnoredShortcode(ctx context.Context, db DB) {
	const query = `
		SELECT id, payload
		FROM task_queue
		WHERE status = 'PENDING'
		FOR UPDATE
	`
	// argus:ignore-a21 sequential maintenance lock
	_ = db.QueryRow(ctx, query)
}
