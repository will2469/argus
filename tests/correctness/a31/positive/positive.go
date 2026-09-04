package positive

import (
	"context"
)

// DB represents a database and transaction execution interface.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	ExecuteTx(ctx context.Context, fn func(tx DB) error) error
	BeginFunc(ctx context.Context, fn func(tx DB) error) error
}

// P1: Obvious Violation — direct UPDATE in transaction without audit call.
func P1_Obvious(ctx context.Context, db DB) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "UPDATE users SET status = 'ACTIVE' WHERE id = 1") // want `\[ARGUS-A31\] database mutation inside transaction is missing required audit trail logging`
		return err
	})
}

// P2: Direct INSERT in BeginFunc transaction without audit logging.
func P2_InsertWithoutAudit(ctx context.Context, db DB) error {
	return db.BeginFunc(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "INSERT INTO orders (user_id, total) VALUES ($1, $2)", 1, 100) // want `\[ARGUS-A31\] database mutation inside transaction is missing required audit trail logging`
		return err
	})
}

// P3: Direct DELETE in transaction without audit logging.
func P3_DeleteWithoutAudit(ctx context.Context, db DB) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		_, err := tx.Exec(ctx, "DELETE FROM accounts WHERE id = 1") // want `\[ARGUS-A31\] database mutation inside transaction is missing required audit trail logging`
		return err
	})
}

// P4: Interprocedural helper call with mutation and missing audit log.
func P4_Interprocedural(ctx context.Context, db DB) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		return performOrderUpdate(ctx, tx)
	})
}

func performOrderUpdate(ctx context.Context, tx DB) error {
	_, err := tx.Exec(ctx, "UPDATE order_items SET quantity = 2 WHERE id = 10") // want `\[ARGUS-A31\] database mutation inside transaction is missing required audit trail logging`
	return err
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, db DB) error {
	return db.ExecuteTx(ctx, func(tx DB) error {
		// argus:ignore ARGUS-A31 system maintenance automated batch
		_, err := tx.Exec(ctx, "DELETE FROM dead_orders WHERE id = 1")
		return err
	})
}
