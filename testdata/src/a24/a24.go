package a24

import (
	"context"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

// 1. Safe SELECT on multi-tenant table with tenant_id predicate (Compliant)
func SafeSelectWithTenant(ctx context.Context, db DB, tenantID string) (any, error) {
	const query = "SELECT id, email, name FROM users WHERE tenant_id = $1 AND status = 'ACTIVE'"
	return db.Query(ctx, query, tenantID)
}

// 2. Safe UPDATE on multi-tenant table with tenant_id predicate (Compliant)
func SafeUpdateWithTenant(ctx context.Context, db DB, tenantID string, id string) (any, error) {
	const query = "UPDATE orders SET status = 'DONE' WHERE tenant_id = $1 AND id = $2"
	return db.Exec(ctx, query, tenantID, id)
}

// 3. Safe DELETE on multi-tenant table with tenant_id predicate (Compliant)
func SafeDeleteWithTenant(ctx context.Context, db DB, tenantID string, id string) (any, error) {
	const query = "DELETE FROM accounts WHERE tenant_id = $1 AND id = $2"
	return db.Exec(ctx, query, tenantID, id)
}

// 4. Safe query on non-tenant reference table (Compliant)
func SafeNonTenantQuery(ctx context.Context, db DB) (any, error) {
	const query = "SELECT code, name FROM lookup_countries WHERE active = true"
	return db.Query(ctx, query)
}

// 5. Safe query under verified RLS session setup (Compliant)
func SafeRLSTransaction(ctx context.Context, db DB, tenantID string) (any, error) {
	if _, err := db.Exec(ctx, "SET LOCAL app.tenant_id = $1", tenantID); err != nil {
		return nil, err
	}
	return db.Query(ctx, "SELECT id, email, name FROM users")
}

// 6. Unsafe SELECT missing tenant_id predicate on users (Violation)
func UnsafeSelectMissingTenant(ctx context.Context, db DB) (any, error) {
	const query = "SELECT id, email, name FROM users WHERE status = 'ACTIVE'"
	return db.Query(ctx, query) // want `\[ARGUS-A24\] query on multi-tenant table 'users' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
}

// 7. Unsafe UPDATE missing tenant_id predicate on orders (Violation)
func UnsafeUpdateMissingTenant(ctx context.Context, db DB, cutoff string) (any, error) {
	const query = "UPDATE orders SET status = 'EXPIRED' WHERE created_at < $1"
	return db.Exec(ctx, query, cutoff) // want `\[ARGUS-A24\] UPDATE on multi-tenant table 'orders' missing 'tenant_id' predicate; risk of cross-tenant data mutation \(CWE-284, OWASP API1:2023 BOLA\)`
}

// 8. Unsafe DELETE missing tenant_id predicate on accounts (Violation)
func UnsafeDeleteMissingTenant(ctx context.Context, db DB) (any, error) {
	const query = "DELETE FROM accounts WHERE status = 'DELETED'"
	return db.Exec(ctx, query) // want `\[ARGUS-A24\] DELETE on multi-tenant table 'accounts' missing 'tenant_id' predicate; risk of cross-tenant data deletion \(CWE-284, OWASP API1:2023 BOLA\)`
}

// 9. Ignored via directive
func IgnoredQuery(ctx context.Context, db DB) (any, error) {
	const query = "SELECT COUNT(*) FROM users GROUP BY country"
	// argus:ignore ARGUS-A24 global analytics rollup
	return db.Query(ctx, query)
}

// 10. Ignored via shortcode
func IgnoredShortcode(ctx context.Context, db DB) (any, error) {
	const query = "SELECT id FROM users WHERE status = 'PENDING'"
	// argus:ignore-a24 telemetry aggregation
	return db.Query(ctx, query)
}

// 11. Unsafe SELECT with disjunctive OR tenant predicate (Violation)
func UnsafeSelectDisjunctiveOR(ctx context.Context, db DB, id, tenantID string) (any, error) {
	const query = "SELECT id, email, name FROM users WHERE id = $1 OR tenant_id = $2"
	return db.Query(ctx, query, id, tenantID) // want `\[ARGUS-A24\] query on multi-tenant table 'users' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
}

// 12. Unsafe SELECT using NullTest pseudo-predicate (Violation)
func UnsafeSelectNullTest(ctx context.Context, db DB) (any, error) {
	const query = "SELECT id, email FROM users WHERE tenant_id IS NOT NULL"
	return db.Query(ctx, query) // want `\[ARGUS-A24\] query on multi-tenant table 'users' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
}

// 13. Unsafe SELECT using non-isolating relational operator (Violation)
func UnsafeSelectRelationalOp(ctx context.Context, db DB) (any, error) {
	const query = "SELECT id, email FROM users WHERE tenant_id > 0"
	return db.Query(ctx, query) // want `\[ARGUS-A24\] query on multi-tenant table 'users' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
}

// 14. Unsafe SELECT using inequality operator (Violation)
func UnsafeSelectNotEqual(ctx context.Context, db DB, tenantID string) (any, error) {
	const query = "SELECT id, email FROM users WHERE tenant_id != $1"
	return db.Query(ctx, query, tenantID) // want `\[ARGUS-A24\] query on multi-tenant table 'users' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
}

// 15. Unsafe multi-table JOIN leaking unconstrained second tenant table (Violation)
func UnsafeJoinMissingSecondTableTenant(ctx context.Context, db DB, tenantID string) (any, error) {
	const query = "SELECT u.id, a.name FROM users u JOIN accounts a ON a.id = u.account_id WHERE u.tenant_id = $1"
	return db.Query(ctx, query, tenantID) // want `\[ARGUS-A24\] query on multi-tenant table 'accounts' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
}

// 16. Safe multi-table JOIN with both tenant tables properly constrained (Compliant)
func SafeJoinBothTablesIsolated(ctx context.Context, db DB, tenantID string) (any, error) {
	const query = "SELECT u.id, a.name FROM users u JOIN accounts a ON a.id = u.account_id WHERE u.tenant_id = $1 AND a.tenant_id = $1"
	return db.Query(ctx, query, tenantID)
}

// 17. Unsafe query executing BEFORE RLS setup occurs in the same function (Violation)
func UnsafeQueryBeforeRLS(ctx context.Context, db DB, tenantID string) (any, error) {
	res, err := db.Query(ctx, "SELECT id, email, name FROM users") // want `\[ARGUS-A24\] query on multi-tenant table 'users' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
	if err != nil {
		return nil, err
	}
	_, _ = db.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID)
	return res, nil
}

// 18. Unsafe query executing following conditional RLS setup without else branch (Violation)
func UnsafeConditionalRLS(ctx context.Context, db DB, tenantID string, isSpecial bool) (any, error) {
	if isSpecial {
		if _, err := db.Exec(ctx, "SET LOCAL app.tenant_id = $1", tenantID); err != nil {
			return nil, err
		}
	}
	return db.Query(ctx, "SELECT id, email, name FROM users") // want `\[ARGUS-A24\] query on multi-tenant table 'users' missing 'tenant_id' predicate; risk of cross-tenant data breach \(CWE-284, OWASP API1:2023 BOLA\)`
}

// 19. Safe query inside conditional block where RLS setup dominates (Compliant)
func SafeQueryInsideRLSBranch(ctx context.Context, db DB, tenantID string, isSpecial bool) (any, error) {
	if isSpecial {
		if _, err := db.Exec(ctx, "SET LOCAL app.tenant_id = $1", tenantID); err != nil {
			return nil, err
		}
		return db.Query(ctx, "SELECT id, email, name FROM users")
	}
	return nil, nil
}



