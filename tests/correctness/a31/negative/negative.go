package negative

import (
	"context"
)

// DB represents a database and transaction execution interface.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
	ExecuteTx(ctx context.Context, fn func(tx DB) error) error
	BeginFunc(ctx context.Context, fn func(tx DB) error) error
}

// AuditLogger represents an authorized audit log recorder.
type AuditLogger interface {
	SaveTx(ctx context.Context, tx DB, event string) error
	RecordTx(ctx context.Context, event string) error
	LogAuditEvent(ctx context.Context, action string) error
}

// N1: Compliant — mutation accompanied by SaveTx call.
func N1_SaveTxAudit(ctx context.Context, db DB, auditor AuditLogger) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "UPDATE users SET status = 'ACTIVE' WHERE id = 1")
		if err != nil {
			return err
		}
		return auditor.SaveTx(ctx, tx, "USER_STATUS_UPDATED")
	})
}

// N2: Compliant — insertion accompanied by RecordTx call.
func N2_RecordTxAudit(ctx context.Context, db DB, auditor AuditLogger) error {
	return db.BeginFunc(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "INSERT INTO orders (user_id, total) VALUES ($1, $2)", 1, 100)
		if err != nil {
			return err
		}
		return auditor.RecordTx(ctx, "ORDER_CREATED")
	})
}

// N3: Compliant — deletion accompanied by LogAuditEvent call.
func N3_LogAuditEvent(ctx context.Context, db DB, auditor AuditLogger) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "DELETE FROM accounts WHERE id = 1")
		if err != nil {
			return err
		}
		return auditor.LogAuditEvent(ctx, "ACCOUNT_DELETED")
	})
}

// N4: Compliant — mutation on exempt table 'sessions'.
func N4_ExemptSessions(ctx context.Context, db DB) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "UPDATE sessions SET last_seen = NOW() WHERE id = 1")
		return err
	})
}

// N5: Compliant — deletion on exempt table 'cache'.
func N5_ExemptCache(ctx context.Context, db DB) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "DELETE FROM cache WHERE key = 'expired'")
		return err
	})
}

// N6: Compliant — read-only queries inside transaction.
func N6_ReadOnlyQueries(ctx context.Context, db DB) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Query(ctx, "SELECT * FROM users WHERE status = 'ACTIVE'")
		return err
	})
}

// N7: Compliant — direct append to audit table.
func N7_DirectAuditInsert(ctx context.Context, db DB) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "UPDATE users SET role = 'admin' WHERE id = 1")
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "INSERT INTO audit_logs (action) VALUES ('ROLE_CHANGED')")
		return err
	})
}
