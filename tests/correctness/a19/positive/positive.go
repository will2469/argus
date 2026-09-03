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

// P1: Obvious Violation — Unbounded query on high-cardinality audit_logs without LIMIT.
func P1_Obvious(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id, action, payload FROM audit_logs WHERE tenant_id = $1") // want `\[ARGUS-A19\] unbounded query on high-cardinality table "audit_logs" without LIMIT or keyset pagination; risk of buffer cache eviction and OOM crash \(CWE-400\)`
}

// P2: Indirect Violation — Unbounded query on transactions table without LIMIT.
func P2_Indirect(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id, amount FROM transactions WHERE status = 'pending'") // want `\[ARGUS-A19\] unbounded query on high-cardinality table "transactions" without LIMIT or keyset pagination; risk of buffer cache eviction and OOM crash \(CWE-400\)`
}

// P3: Helper Violation — Helper querying events table without LIMIT.
func P3_Helper(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id, type FROM events WHERE created_at > $1") // want `\[ARGUS-A19\] unbounded query on high-cardinality table "events" without LIMIT or keyset pagination; risk of buffer cache eviction and OOM crash \(CWE-400\)`
}

// P4: Nested Violation — Aggregate with GROUP BY on orders and no LIMIT.
func P4_Nested(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT status, COUNT(*) FROM orders GROUP BY status") // want `\[ARGUS-A19\] unbounded query on high-cardinality table "orders" without LIMIT or keyset pagination; risk of buffer cache eviction and OOM crash \(CWE-400\)`
}

// P5: Alias Violation — Aliased high-cardinality table activity_logs without LIMIT.
func P5_Alias(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT a.id, a.msg FROM activity_logs a WHERE a.level = 'error'") // want `\[ARGUS-A19\] unbounded query on high-cardinality table "activity_logs" without LIMIT or keyset pagination; risk of buffer cache eviction and OOM crash \(CWE-400\)`
}

// P_Ignored: Suppressed violation using canonical shortcode.
func P_Ignored(ctx context.Context, db DB) {
	// argus:ignore-a19 monthly archive dump
	_, _ = db.Query(ctx, "SELECT id, payload FROM audit_logs")
}
