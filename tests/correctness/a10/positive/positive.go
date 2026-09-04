package positive

import (
	"context"
	"database/sql"
)

type Pool interface {
	Begin(ctx context.Context) (*sql.Tx, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

type Helper struct{}

func (Helper) WithTx(ctx context.Context, pool Pool, fn func(*sql.Tx) error, opts ...*sql.TxOptions) error {
	return nil
}

var argus Helper

//
//
//
//
//
//
//
//
//
//
//

// P1: Obvious Violation — default Begin modifying critical table "balances".
func P1_Obvious(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("UPDATE balances SET amount = amount - 100 WHERE id = 1")
	return tx.Commit()
}

// P2: Indirect Violation — WithTx helper without isolation options modifying critical table "inventory".
func P2_Indirect(ctx context.Context, pool Pool) error {
	return argus.WithTx(ctx, pool, func(tx *sql.Tx) error { // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
		_, err := tx.Exec("UPDATE inventory SET sisa = sisa - 1 WHERE id = 1"); return err
	})
}

// P3: Helper Violation — Begin modifying critical table "kuota".
func P3_Helper(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("UPDATE kuota SET sisa = sisa - 1 WHERE id = 1")
	return tx.Commit()
}

// P4: Nested Violation — Begin modifying critical table "rekening".
func P4_Nested(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("UPDATE rekening SET saldo = saldo - 500 WHERE id = 1")
	return tx.Commit()
}

// P5: Alias Violation — Begin modifying critical table "saldo".
func P5_Alias(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("UPDATE saldo SET amount = amount - 50 WHERE id = 1")
	return tx.Commit()
}

// P6: Unrelated Row Lock — Row lock on audit_log does NOT protect write to critical table "balances".
func P6_UnrelatedRowLock(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Query("SELECT * FROM audit_log WHERE id = 1 FOR UPDATE")
	_, _ = tx.Exec("UPDATE balances SET amount = amount - 100 WHERE id = 1")
	return tx.Commit()
}

// P7: Unrelated Advisory Lock — Advisory lock for "audit_sync" does NOT protect critical table "balances".
func P7_UnrelatedAdvisoryLock(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("SELECT pg_advisory_xact_lock(999)")
	_, _ = tx.Exec("UPDATE balances SET amount = amount - 100 WHERE id = 1")
	return tx.Commit()
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, pool Pool) error {
	// argus:ignore ARGUS-A10 manual administrative repair script
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("UPDATE balances SET amount = 500 WHERE id = 1")
	return tx.Commit()
}
