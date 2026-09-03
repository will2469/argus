package negative

import (
	"context"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

// N1: Obvious Safe — Safe SELECT on multi-tenant table with tenant_id predicate.
func N1_ObviousSafe(ctx context.Context, db DB, tenantID string) (any, error) {
	const query = "SELECT id, email, name FROM users WHERE tenant_id = $1 AND status = 'ACTIVE'"
	return db.Query(ctx, query, tenantID)
}

// N2: Legitimate Idiom — Safe UPDATE on multi-tenant table with tenant_id predicate.
func N2_LegitimateIdiom(ctx context.Context, db DB, tenantID string, id string) (any, error) {
	const query = "UPDATE orders SET status = 'DONE' WHERE tenant_id = $1 AND id = $2"
	return db.Exec(ctx, query, tenantID, id)
}

// N3: Unrelated API — Safe query on non-tenant reference table.
func N3_UnrelatedAPI(ctx context.Context, db DB) (any, error) {
	const query = "SELECT code, name FROM lookup_countries WHERE active = true"
	return db.Query(ctx, query)
}

// N4: Verified RLS — Safe query under verified RLS session setup.
func N4_VerifiedRLS(ctx context.Context, db DB, tenantID string) (any, error) {
	if _, err := db.Exec(ctx, "SET LOCAL app.tenant_id = $1", tenantID); err != nil {
		return nil, err
	}
	return db.Query(ctx, "SELECT id, email, name FROM users")
}

// N5: Multi-table Isolated — Safe multi-table JOIN with all tenant tables constrained.
func N5_MultiTableIsolated(ctx context.Context, db DB, tenantID string) (any, error) {
	const query = "SELECT u.id, a.name FROM users u JOIN accounts a ON a.id = u.account_id WHERE u.tenant_id = $1 AND a.tenant_id = $1"
	return db.Query(ctx, query, tenantID)
}
