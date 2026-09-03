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

// P1: Obvious Violation — Unsafe queue polling with blocking FOR UPDATE.
func P1_Obvious(ctx context.Context, db DB) {
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

// P2: Indirect Violation — Unsafe multi-row scan with blocking FOR NO KEY UPDATE.
func P2_Indirect(ctx context.Context, db DB, tenantID string) {
	const query = `
		SELECT id, amount
		FROM pending_payments
		WHERE tenant_id = $1
		LIMIT 10
		FOR NO KEY UPDATE
	`
	_, _ = db.Query(ctx, query, tenantID) // want `\[ARGUS-A21\] blocking row-level lock \(FOR UPDATE / FOR NO KEY UPDATE\) without SKIP LOCKED or NOWAIT; risk of worker lock convoy \(CWE-662, CWE-833\)`
}

// P3: Helper Violation — Helper querying jobs queue with blocking FOR UPDATE.
func P3_Helper(ctx context.Context, db DB) {
	const query = "SELECT id FROM jobs WHERE state = 'READY' FOR UPDATE"
	_, _ = db.Query(ctx, query) // want `\[ARGUS-A21\] blocking row-level lock \(FOR UPDATE / FOR NO KEY UPDATE\) without SKIP LOCKED or NOWAIT; risk of worker lock convoy \(CWE-662, CWE-833\)`
}

// P4: Nested Violation — Inside conditional branch with blocking FOR UPDATE.
func P4_Nested(ctx context.Context, db DB, active bool) {
	if active {
		const query = "SELECT id FROM events WHERE status = 'QUEUED' FOR UPDATE"
		_, _ = db.Query(ctx, query) // want `\[ARGUS-A21\] blocking row-level lock \(FOR UPDATE / FOR NO KEY UPDATE\) without SKIP LOCKED or NOWAIT; risk of worker lock convoy \(CWE-662, CWE-833\)`
	}
}

// P5: Alias Violation — Multi-row task table with blocking FOR NO KEY UPDATE.
func P5_Alias(ctx context.Context, db DB) {
	const query = "SELECT id FROM batch_tasks WHERE processed = false FOR NO KEY UPDATE"
	_, _ = db.Query(ctx, query) // want `\[ARGUS-A21\] blocking row-level lock \(FOR UPDATE / FOR NO KEY UPDATE\) without SKIP LOCKED or NOWAIT; risk of worker lock convoy \(CWE-662, CWE-833\)`
}

// P_Ignored: Suppressed violation using canonical shortcode.
func P_Ignored(ctx context.Context, db DB) {
	const query = `
		SELECT id, payload
		FROM task_queue
		WHERE status = 'PENDING'
		FOR UPDATE
	`
	// argus:ignore-a21 sequential maintenance lock
	_ = db.QueryRow(ctx, query)
}
