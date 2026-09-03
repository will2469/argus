package positive

import (
	"context"
)

// DBExecutor represents a standard database interface for queries.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

// P1: Obvious Violation — direct DELETE statement targeting audit log table.
func P1_Obvious(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "DELETE FROM audit_logs WHERE id = '1'") // want `\[ARGUS-A05\] forbidden DELETE on audit table "audit_logs"`
	return err
}

// P2: Indirect Violation — UPDATE query assembled into local variable.
func P2_Indirect(ctx context.Context, db DBExecutor) error {
	query := "UPDATE audit_logs SET action = 'ALTERED' WHERE id = '1'"
	_, err := db.Exec(ctx, query) // want `\[ARGUS-A05\] forbidden UPDATE on audit table "audit_logs"`
	return err
}

// P3: Helper Violation — TRUNCATE on audit table inside internal helper subroutine.
func P3_Helper(ctx context.Context, db DBExecutor) error {
	return executeTruncateHelper(ctx, db)
}

func executeTruncateHelper(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "TRUNCATE TABLE audit_logs") // want `\[ARGUS-A05\] forbidden TRUNCATE on audit table "audit_logs"`
	return err
}

// P4: Nested Violation — CTE containing DELETE on audit table.
func P4_Nested(ctx context.Context, db DBExecutor) error {
	query := `
		WITH deleted AS (
			DELETE FROM audit_logs WHERE id = '1'
		)
		SELECT 1
	`
	_, err := db.Exec(ctx, query) // want `\[ARGUS-A05\] forbidden DELETE on audit table "audit_logs"`
	return err
}

// P5: Alias Violation — mutation targeting security_events via aliased executor.
type SecurityDB = DBExecutor

func P5_Alias(ctx context.Context, db SecurityDB) error {
	_, err := db.Exec(ctx, "UPDATE security_events SET severity = 'LOW'") // want `\[ARGUS-A05\] forbidden UPDATE on audit table "security_events"`
	return err
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, db DBExecutor) error {
	// argus:ignore ARGUS-A05 maintenance data deduplication
	_, err := db.Exec(ctx, "DELETE FROM audit_logs WHERE action = 'DUPLICATE'")
	return err
}
