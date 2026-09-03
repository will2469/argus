package positive

import (
	"context"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

// P1: Obvious Violation — Unsafe SELECT missing tenant_id predicate on users.
func P1_Obvious(ctx context.Context, db DB) (any, error) {
	const query = "SELECT id, email, name FROM users WHERE status = 'ACTIVE'"
	return db.Query(ctx, query) // want `\[ARGUS-A24\] query on multi-tenant table 'users' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
}

// P2: Indirect Violation — Unsafe UPDATE missing tenant_id predicate on orders.
func P2_Indirect(ctx context.Context, db DB, cutoff string) (any, error) {
	const query = "UPDATE orders SET status = 'EXPIRED' WHERE created_at < $1"
	return db.Exec(ctx, query, cutoff) // want `\[ARGUS-A24\] UPDATE on multi-tenant table 'orders' missing 'tenant_id' predicate; risk of cross-tenant data mutation \(CWE-284, OWASP API1:2023 BOLA\)`
}

// P3: Helper Violation — Unsafe DELETE missing tenant_id predicate on accounts.
func P3_Helper(ctx context.Context, db DB) (any, error) {
	const query = "DELETE FROM accounts WHERE status = 'DELETED'"
	return db.Exec(ctx, query) // want `\[ARGUS-A24\] DELETE on multi-tenant table 'accounts' missing 'tenant_id' predicate; risk of cross-tenant data deletion \(CWE-284, OWASP API1:2023 BOLA\)`
}

// P4: Nested Violation — Unsafe SELECT with disjunctive OR tenant predicate.
func P4_Nested(ctx context.Context, db DB, id, tenantID string) (any, error) {
	const query = "SELECT id, email, name FROM users WHERE id = $1 OR tenant_id = $2"
	return db.Query(ctx, query, id, tenantID) // want `\[ARGUS-A24\] query on multi-tenant table 'users' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
}

// P5: Alias Violation — Unsafe multi-table JOIN leaking unconstrained second tenant table.
func P5_Alias(ctx context.Context, db DB, tenantID string) (any, error) {
	const query = "SELECT u.id, a.name FROM users u JOIN accounts a ON a.id = u.account_id WHERE u.tenant_id = $1"
	return db.Query(ctx, query, tenantID) // want `\[ARGUS-A24\] query on multi-tenant table 'accounts' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
}

// P_Ignored: Suppressed violation using canonical shortcode.
func P_Ignored(ctx context.Context, db DB) (any, error) {
	const query = "SELECT id FROM users WHERE status = 'PENDING'"
	// argus:ignore-a24 telemetry aggregation
	return db.Query(ctx, query)
}
