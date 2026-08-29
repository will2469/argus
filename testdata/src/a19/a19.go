package a19

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

func Cases(ctx context.Context, db DB) {
	// 1. Unbounded query on audit_logs (Violation)
	_, _ = db.Query(ctx, "SELECT id, action, payload FROM audit_logs WHERE tenant_id = $1") // want `\[ARGUS-A19\] unbounded query on high-cardinality table "audit_logs" without LIMIT or keyset pagination; risk of buffer cache eviction and OOM crash \(CWE-400\)`

	// 2. Bounded query with LIMIT (Compliant)
	_, _ = db.Query(ctx, "SELECT id, action, payload FROM audit_logs WHERE tenant_id = $1 LIMIT 50")

	// 3. Keyset pagination with LIMIT (Compliant)
	_, _ = db.Query(ctx, "SELECT id, action FROM audit_logs WHERE tenant_id = $1 AND id < $2 ORDER BY id DESC LIMIT 100")

	// 4. Scalar aggregate without GROUP BY (Compliant)
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1")

	// 5. Aggregate with GROUP BY and no LIMIT (Violation)
	_, _ = db.Query(ctx, "SELECT action, COUNT(*) FROM audit_logs GROUP BY action") // want `\[ARGUS-A19\] unbounded query on high-cardinality table "audit_logs" without LIMIT or keyset pagination; risk of buffer cache eviction and OOM crash \(CWE-400\)`

	// 6. Point lookup by Primary Key (Compliant)
	_ = db.QueryRow(ctx, "SELECT id, name FROM users WHERE id = $1")

	// 7. Non-high-cardinality reference table (Compliant)
	_, _ = db.Query(ctx, "SELECT id, name FROM ref_agama")

	// 8. FETCH FIRST clause (Compliant)
	_, _ = db.Query(ctx, "SELECT id, action FROM audit_logs FETCH FIRST 100 ROWS ONLY")

	// 9. Ignored query via full directive
	// argus:ignore ARGUS-A19 offline analytical export worker
	_, _ = db.Query(ctx, "SELECT id, payload FROM audit_logs")

	// 10. Ignored query via shortcode directive
	// argus:ignore-a19 monthly archive dump
	_, _ = db.Query(ctx, "SELECT id, payload FROM audit_logs")
}
