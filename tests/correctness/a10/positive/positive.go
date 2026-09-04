package positive

import (
	"context"
)

type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Query(ctx context.Context, sql string, args ...any) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type TxOptions struct {
	IsoLevel string
}

type Pool interface {
	Begin(ctx context.Context) (Tx, error)
	BeginTx(ctx context.Context, opts TxOptions) (Tx, error)
}

type Helper struct{}

func (Helper) WithTx(ctx context.Context, pool Pool, fn func(Tx) error, opts ...TxOptions) error {
	return nil
}

var argus Helper

// P1: Obvious Violation — default Begin modifying critical table "balances".
func P1_Obvious(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE balances SET amount = amount - 100 WHERE id = 1")
	return tx.Commit(ctx)
}

// P2: Indirect Violation — WithTx helper without isolation options modifying critical table "inventory".
func P2_Indirect(ctx context.Context, pool Pool) error {
	return argus.WithTx(ctx, pool, func(tx Tx) error { // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
		return tx.Exec(ctx, "UPDATE inventory SET sisa = sisa - 1 WHERE id = 1")
	})
}

// P3: Helper Violation — Begin modifying critical table "kuota".
func P3_Helper(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE kuota SET sisa = sisa - 1 WHERE id = 1")
	return tx.Commit(ctx)
}

// P4: Nested Violation — Begin modifying critical table "rekening".
func P4_Nested(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE rekening SET saldo = saldo - 500 WHERE id = 1")
	return tx.Commit(ctx)
}

// P5: Alias Violation — Begin modifying critical table "saldo".
func P5_Alias(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE saldo SET amount = amount - 50 WHERE id = 1")
	return tx.Commit(ctx)
}

// P6: Unrelated Row Lock — Row lock on audit_log does NOT protect write to critical table "balances".
func P6_UnrelatedRowLock(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Query(ctx, "SELECT * FROM audit_log WHERE id = 1 FOR UPDATE")
	_ = tx.Exec(ctx, "UPDATE balances SET amount = amount - 100 WHERE id = 1")
	return tx.Commit(ctx)
}

// P7: Unrelated Advisory Lock — Advisory lock for "audit_sync" does NOT protect critical table "balances".
func P7_UnrelatedAdvisoryLock(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(999)")
	_ = tx.Exec(ctx, "UPDATE balances SET amount = amount - 100 WHERE id = 1")
	return tx.Commit(ctx)
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, pool Pool) error {
	// argus:ignore ARGUS-A10 manual administrative repair script
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE balances SET amount = 500 WHERE id = 1")
	return tx.Commit(ctx)
}
