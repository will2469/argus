package a05

import (
	"context"
)

type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func SafeInsert(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "INSERT INTO audit_logs (id, action) VALUES ($1, $2)", "1", "LOGIN")
	return err
}

func SafeSelect(ctx context.Context, db DBExecutor) error {
	_, err := db.Query(ctx, "SELECT id, action FROM audit_logs WHERE id = $1", "1")
	return err
}

func BadUpdate(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "UPDATE audit_logs SET action = 'HACK' WHERE id = '1'") // want `\[ARGUS-A05\] forbidden UPDATE on audit table "audit_logs"`
	return err
}

func BadDelete(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "DELETE FROM audit_logs WHERE id = '1'") // want `\[ARGUS-A05\] forbidden DELETE on audit table "audit_logs"`
	return err
}

func BadTruncate(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "TRUNCATE TABLE audit_logs") // want `\[ARGUS-A05\] forbidden TRUNCATE on audit table "audit_logs"`
	return err
}

func BadMerge(ctx context.Context, db DBExecutor) error {
	query := `
		MERGE INTO audit_logs a
		USING temp_logs t ON a.id = t.id
		WHEN MATCHED THEN
			UPDATE SET action = t.action
	`
	_, err := db.Exec(ctx, query) // want `\[ARGUS-A05\] forbidden MERGE on audit table "audit_logs"`
	return err
}

func BadCTE(ctx context.Context, db DBExecutor) error {
	query := `
		WITH deleted AS (
			DELETE FROM audit_logs WHERE id = '1'
		)
		SELECT 1
	`
	_, err := db.Exec(ctx, query) // want `\[ARGUS-A05\] forbidden DELETE on audit table "audit_logs"`
	return err
}

func IgnoredTampering(ctx context.Context, db DBExecutor) error {
	// argus:ignore ARGUS-A05 maintenance data deduplication
	_, err := db.Exec(ctx, "DELETE FROM audit_logs WHERE action = 'DUPLICATE'")
	return err
}
