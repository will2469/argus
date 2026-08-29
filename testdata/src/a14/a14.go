package a14

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

func TestCases(ctx context.Context, db DB) {
	// 1. Plain SELECT *
	_, _ = db.Query(ctx, "SELECT * FROM users") // want `\[ARGUS-A14\] Forbidden 'SELECT \*' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure \(CWE-200\)`

	// 2. Table alias wildcard
	_, _ = db.Query(ctx, "SELECT u.*, p.name FROM users u JOIN profiles p ON u.id = p.user_id") // want `\[ARGUS-A14\] Forbidden 'SELECT \*' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure \(CWE-200\)`

	// 3. Wildcard in CTE
	_, _ = db.Query(ctx, "WITH active AS (SELECT * FROM users) SELECT id FROM active") // want `\[ARGUS-A14\] Forbidden 'SELECT \*' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure \(CWE-200\)`

	// 4. Compliant explicit projection
	_, _ = db.Query(ctx, "SELECT id, nik, nama_lengkap FROM users WHERE is_active = true")

	// 5. Compliant COUNT(*) aggregate
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM users")

	// 6. Compliant EXISTS (SELECT * ...)
	_, _ = db.Query(ctx, "SELECT id FROM users WHERE EXISTS (SELECT * FROM profiles WHERE profiles.user_id = users.id)")

	// 7. Compliant NOT EXISTS (SELECT * ...)
	_, _ = db.Query(ctx, "SELECT id FROM users WHERE NOT EXISTS (SELECT * FROM profiles WHERE profiles.user_id = users.id)")

	// 8. Ignored call via canonical shortcode
	// argus:ignore-a14 offline disaster recovery full row table dump
	_, _ = db.Query(ctx, "SELECT * FROM audit_logs_archive")
}
