package adversarial

import (
	"context"
)

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

// A1: Branch — conditional deletion on audit logs inside an if-statement.
func A1_Branch(ctx context.Context, db DBExecutor, shouldPurge bool) error {
	if shouldPurge {
		_, err := db.Exec(ctx, "DELETE FROM audit_logs WHERE id = '1'")
		return err
	}
	return nil
}

// A2: Reassignment — variable reassigned from safe SELECT to forbidden UPDATE.
func A2_Reassignment(ctx context.Context, db DBExecutor) error {
	q := "SELECT * FROM audit_logs"
	_ = q
	q = "UPDATE audit_logs SET action = 'ALTERED' WHERE id = '1'"
	_, err := db.Exec(ctx, q)
	return err
}

// A3: Alias — raw SQL query referenced through variable indirection.
func A3_Alias(ctx context.Context, db DBExecutor) error {
	rawSQL := "TRUNCATE TABLE audit_logs"
	alias := rawSQL
	_, err := db.Exec(ctx, alias)
	return err
}

// A4: Wrapper — administration wrapper struct executing DROP TABLE on audit logs.
type AuditAdmin struct {
	db DBExecutor
}

func (a *AuditAdmin) DropAuditTable(ctx context.Context) error {
	_, err := a.db.Exec(ctx, "DROP TABLE audit_logs")
	return err
}

// A5: Nested Function — closure capturing executor and executing MERGE query.
func A5_NestedFunction(ctx context.Context, db DBExecutor) error {
	mergeSQL := `
		MERGE INTO audit_logs a
		USING temp_logs t ON a.id = t.id
		WHEN MATCHED THEN
			UPDATE SET action = t.action
	`
	purge := func() error {
		_, err := db.Exec(ctx, mergeSQL)
		return err
	}
	return purge()
}

// A6: Generic — generic auditor executing deletion on security_events.
type Auditor[T any] struct {
	db DBExecutor
}

func (a *Auditor[T]) PurgeEvents(ctx context.Context) error {
	_, err := a.db.Exec(ctx, "DELETE FROM security_events WHERE severity = 'DEBUG'")
	return err
}

// A7: Interface — dynamic interface assertion before executing UPDATE.
func A7_Interface(ctx context.Context, client any) error {
	if exec, ok := client.(DBExecutor); ok {
		_, err := exec.Exec(ctx, "UPDATE audit_logs SET action = 'TAMPERED'")
		return err
	}
	return nil
}
