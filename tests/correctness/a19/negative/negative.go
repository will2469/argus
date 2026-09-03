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

// N1: Obvious Safe — bounded query with explicit LIMIT clause.
func N1_ObviousSafe(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id, action, payload FROM audit_logs WHERE tenant_id = $1 LIMIT 50")
}

// N2: Legitimate Idiom — keyset pagination with explicit LIMIT clause.
func N2_LegitimateIdiom(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id, action FROM audit_logs WHERE tenant_id = $1 AND id < $2 ORDER BY id DESC LIMIT 100")
}

// N3: Unrelated API — reference/catalog table not in high-cardinality list.
func N3_UnrelatedAPI(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id, name FROM ref_agama")
}

// N4: Point Lookup — lookup on primary key / point lookup column.
func N4_PointLookup(ctx context.Context, db DB) {
	_ = db.QueryRow(ctx, "SELECT id, name FROM users WHERE id = $1")
}

// N5: Static Constant — SQL standard FETCH FIRST ROWS ONLY pagination.
func N5_FetchFirst(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id, action FROM audit_logs FETCH FIRST 100 ROWS ONLY")
}

// N6: Scalar Aggregate — scalar aggregate without GROUP BY is guaranteed single-row.
func N6_ScalarAggregate(ctx context.Context, db DB) {
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1")
}
