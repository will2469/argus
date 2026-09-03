package negative

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

// N1: Obvious Safe — explicit Serializable isolation level.
func N1_Serializable(ctx context.Context, pool Pool) error {
	opts := TxOptions{IsoLevel: Serializable}
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE balances SET amount = amount - 100 WHERE id = 1")
	return tx.Commit(ctx)
}

// N2: Legitimate Idiom — explicit RepeatableRead isolation level.
func N2_RepeatableRead(ctx context.Context, pool Pool) error {
	tx, err := pool.BeginTx(ctx, TxOptions{IsoLevel: RepeatableRead})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE inventory SET sisa = sisa - 1 WHERE id = 1")
	return tx.Commit(ctx)
}

// N3: Unrelated API — modification of non-critical table.
func N3_NonCriticalTable(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE user_preferences SET theme = 'dark' WHERE id = 1")
	return tx.Commit(ctx)
}

// N4: Sanitized / Row Lock — pessimistic row lock using SELECT FOR UPDATE.
func N4_RowLock(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Query(ctx, "SELECT amount FROM balances WHERE id = 1 FOR UPDATE")
	_ = tx.Exec(ctx, "UPDATE balances SET amount = amount - 50 WHERE id = 1")
	return tx.Commit(ctx)
}

// N5: Advisory Lock Wrapped — safe advisory lock enclosure.
func N5_AdvisoryLock(ctx context.Context, pool Pool) error {
	return argus.WithAdvisoryLock(ctx, "lock:inventory", func() error {
		return argus.WithTx(ctx, pool, func(tx Tx) error {
			return tx.Exec(ctx, "UPDATE inventory SET sisa = sisa - 1 WHERE id = 1")
		})
	})
}
