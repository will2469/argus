package adversarial

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

// A1: Branch — conditional write to critical table inside branch without strong isolation.
func A1_Branch(ctx context.Context, pool Pool, isBonus bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if isBonus {
		_ = tx.Exec(ctx, "UPDATE balances SET amount = amount + 10 WHERE id = 1")
	}
	return tx.Commit(ctx)
}

// A2: Reassignment — query variable reassigned to critical table update.
func A2_Reassignment(ctx context.Context, pool Pool) error {
	q := "UPDATE preferences SET theme = 'dark'"
	_ = q
	q = "UPDATE balances SET amount = 0 WHERE id = 1"
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, q)
	return tx.Commit(ctx)
}

// A3: Alias — aliased query string targeting critical table.
func A3_Alias(ctx context.Context, pool Pool) error {
	raw := "UPDATE accounts SET balance = balance - 10 WHERE id = 1"
	alias := raw
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, alias)
	return tx.Commit(ctx)
}

// A4: Wrapper — repository struct method with default Begin on critical table.
type WalletRepo struct {
	pool Pool
}

func (w *WalletRepo) Deduct(ctx context.Context) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE balances SET amount = amount - 1 WHERE id = 1")
	return tx.Commit(ctx)
}

// A5: Nested Function — closure modifying critical table.
func A5_NestedFunction(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	deduct := func() error {
		return tx.Exec(ctx, "UPDATE kuota SET sisa = 0 WHERE id = 1")
	}
	_ = deduct()
	return tx.Commit(ctx)
}

// A6: Generic — generic transaction service modifying critical table without isolation options.
type TxService[T any] struct {
	pool Pool
}

func (s *TxService[T]) Run(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE nomor_urut SET counter = counter + 1 WHERE id = 1")
	return tx.Commit(ctx)
}

// A7: Interface / Helper — dynamic WithTx helper invocation without isolation level.
func A7_HelperNoIso(ctx context.Context, pool Pool, h Helper) error {
	return h.WithTx(ctx, pool, func(tx Tx) error {
		return tx.Exec(ctx, "UPDATE balances SET amount = 0 WHERE id = 1")
	})
}

// A8: Non-DB WithTx Helper — orderService.WithTx is a non-DB helper and must NOT trigger false positive.
type OrderService struct{}

func (OrderService) WithTx(ctx context.Context, fn func() error) error {
	return fn()
}

func A8_NonDBWithTxHelper_MustBeSafe(ctx context.Context, svc OrderService) error {
	return svc.WithTx(ctx, func() error {
		_ = "UPDATE balances SET amount = 0 WHERE id = 1"
		return nil
	})
}

// A9: Non-DB WorkerPool — workerPool.Begin is not a database transaction and must NOT trigger false positive.
type WorkerPool struct {
	workers []int
}

func (WorkerPool) Begin() error {
	return nil
}

func A9_WorkerPoolBegin_MustBeSafe(wp WorkerPool) error {
	return wp.Begin()
}

