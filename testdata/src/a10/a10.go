package a10

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

const (
	Serializable   = "Serializable"
	RepeatableRead = "RepeatableRead"
)

type Pool interface {
	Begin(ctx context.Context) (Tx, error)
	BeginTx(ctx context.Context, opts TxOptions) (Tx, error)
}

type Helper struct{}

func (Helper) WithTx(ctx context.Context, pool Pool, fn func(Tx) error, opts ...TxOptions) error {
	return nil
}

func (Helper) WithAdvisoryLock(ctx context.Context, lock string, fn func() error) error {
	return fn()
}

var argus Helper

func SafeSerializableTx(ctx context.Context, pool Pool) error {
	opts := TxOptions{IsoLevel: Serializable}
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_ = tx.Exec(ctx, "UPDATE balances SET amount = amount - 100 WHERE id = 1")
	return tx.Commit(ctx)
}

func SafeRepeatableReadTx(ctx context.Context, pool Pool) error {
	tx, err := pool.BeginTx(ctx, TxOptions{IsoLevel: RepeatableRead})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_ = tx.Exec(ctx, "UPDATE inventory SET sisa = sisa - 1 WHERE id = 1")
	return tx.Commit(ctx)
}

func SafeRowLockTx(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_ = tx.Query(ctx, "SELECT amount FROM balances WHERE id = 1 FOR UPDATE")
	_ = tx.Exec(ctx, "UPDATE balances SET amount = amount - 50 WHERE id = 1")
	return tx.Commit(ctx)
}

func SafeNonCriticalTable(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_ = tx.Exec(ctx, "UPDATE user_preferences SET theme = 'dark' WHERE id = 1")
	return tx.Commit(ctx)
}

func SafeAdvisoryLockWrapped(ctx context.Context, pool Pool) error {
	return argus.WithAdvisoryLock(ctx, "lock:inventory", func() error {
		return argus.WithTx(ctx, pool, func(tx Tx) error {
			return tx.Exec(ctx, "UPDATE inventory SET sisa = sisa - 1 WHERE id = 1")
		})
	})
}

func BadBeginDefault(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx) // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_ = tx.Exec(ctx, "UPDATE balances SET amount = amount - 100 WHERE id = 1")
	return tx.Commit(ctx)
}

func BadWithTxNoIso(ctx context.Context, pool Pool) error {
	return argus.WithTx(ctx, pool, func(tx Tx) error { // want `\[ARGUS-A10\] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock`
		return tx.Exec(ctx, "UPDATE inventory SET sisa = sisa - 1 WHERE id = 1")
	})
}

func IgnoredTx(ctx context.Context, pool Pool) error {
	// argus:ignore ARGUS-A10 manual admin repair script
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_ = tx.Exec(ctx, "UPDATE balances SET amount = 500 WHERE id = 1")
	return tx.Commit(ctx)
}
