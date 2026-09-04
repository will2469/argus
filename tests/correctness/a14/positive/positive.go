package positive

import (
	"context"
	"database/sql"
)

type DB struct{ *sql.DB }

func (DB) Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func (DB) QueryRow(ctx context.Context, sql string, args ...any) *sql.Row {
	return nil
}

func (DB) Exec(ctx context.Context, sql string, args ...any) (sql.Result, error) {
	return nil, nil
}
// P1: Obvious Violation — direct SELECT * wildcard.
func P1_Obvious(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT * FROM users") // want `\[ARGUS-A14\] Forbidden 'SELECT \*' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure \(CWE-200\)`
}

// P2: Indirect Violation — table alias wildcard projection.
func P2_Indirect(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT u.*, p.name FROM users u JOIN profiles p ON u.id = p.user_id") // want `\[ARGUS-A14\] Forbidden 'SELECT \*' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure \(CWE-200\)`
}

// P3: Helper Violation — wildcard in CTE subquery.
func P3_Helper(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "WITH active AS (SELECT * FROM users) SELECT id FROM active") // want `\[ARGUS-A14\] Forbidden 'SELECT \*' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure \(CWE-200\)`
}

// P4: Nested Violation — wildcard in UNION select statement.
func P4_Nested(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id FROM users UNION SELECT * FROM archived_users") // want `\[ARGUS-A14\] Forbidden 'SELECT \*' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure \(CWE-200\)`
}

// P5: Alias Violation — multiple table alias wildcards.
func P5_Alias(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT a.*, b.* FROM table_a a JOIN table_b b ON a.id = b.id") // want `\[ARGUS-A14\] Forbidden 'SELECT \*' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure \(CWE-200\)`
}

// P6: Shadowed Violation — inner scope shadows outer safe query with SELECT *.
func P6_ShadowedSelectStar(ctx context.Context, db DB) {
	query := "SELECT id FROM users"
	_ = query
	if true {
		query := "SELECT * FROM audit"
		_, _ = db.Query(ctx, query) // want `\[ARGUS-A14\] Forbidden 'SELECT \*' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure \(CWE-200\)`
	}
}

var packageQuery = "SELECT id FROM users"

// P7: Package Shadowing — inner function variable shadows safe package var with SELECT *.
func P7_PackageShadowing(ctx context.Context, db DB) {
	_ = packageQuery
	packageQuery := "SELECT * FROM audit"
	_, _ = db.Query(ctx, packageQuery) // want `\[ARGUS-A14\] Forbidden 'SELECT \*' or wildcard column selection detected; explicitly list required columns to prevent TOAST table bloat and data exposure \(CWE-200\)`
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, db DB) {
	// argus:ignore-a14 offline disaster recovery full row table dump
	_, _ = db.Query(ctx, "SELECT * FROM audit_logs_archive")
}
