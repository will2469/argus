package adversarial

import (
	"context"
)

// DB represents a database and transaction execution interface.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	ExecuteTx(ctx context.Context, fn func(tx DB) error) error
	Begin(ctx context.Context) (DB, error)
	Commit() error
}

// AuditRecorder defines an audit recording interface.
type AuditRecorder interface {
	SaveTx(ctx context.Context, tx DB, event string) error
	Save(ctx context.Context, event string) error
}

// A1: Multiple mutations inside transaction with a single final audit call.
func A1_MultipleMutationsWithSingleAudit(ctx context.Context, db DB, auditor AuditRecorder) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		if _, err := tx.Exec(ctx, "UPDATE accounts SET balance = balance - 100 WHERE id = 1"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "UPDATE accounts SET balance = balance + 100 WHERE id = 2"); err != nil {
			return err
		}
		return auditor.SaveTx(ctx, tx, "TRANSFER_COMPLETE")
	})
}

// A2: Audit call encapsulated inside a helper function.
func A2_AuditInHelper(ctx context.Context, db DB, auditor AuditRecorder) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		if _, err := tx.Exec(ctx, "INSERT INTO audit_targets (val) VALUES ('x')"); err != nil {
			return err
		}
		return recordHelper(ctx, auditor)
	})
}

func recordHelper(ctx context.Context, auditor AuditRecorder) error {
	return auditor.Save(ctx, "TARGET_INSERTED")
}

// A3: Conditional mutation inside if block without audit.
func A3_ConditionalMutationWithoutAudit(ctx context.Context, db DB, shouldDelete bool) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		if shouldDelete {
			_, err := tx.Exec(ctx, "DELETE FROM customers WHERE id = 99")
			return err
		}
		return nil
	})
}

// A4: Audit method called before mutation.
func A4_AuditBeforeMutation(ctx context.Context, db DB, auditor AuditRecorder) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		if err := auditor.SaveTx(ctx, tx, "INTENT_UPDATE"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "UPDATE customers SET status = 'PRE_VERIFIED' WHERE id = 1")
		return err
	})
}

// A5: Explicit transaction block without audit call.
func A5_ExplicitTxWithoutAudit(ctx context.Context, db DB) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE users SET verified = true WHERE id = 1")
	if err != nil {
		return err
	}
	return tx.Commit()
}

// A6: Explicit transaction block with audit call.
func A6_ExplicitTxWithAudit(ctx context.Context, db DB, auditor AuditRecorder) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE users SET verified = true WHERE id = 1")
	if err != nil {
		return err
	}
	if err := auditor.SaveTx(ctx, tx, "USER_VERIFIED"); err != nil {
		return err
	}
	return tx.Commit()
}
