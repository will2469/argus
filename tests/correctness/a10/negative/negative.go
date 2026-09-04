package negative

import (
	"context"
	"database/sql"
)

type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Query(ctx context.Context, sql string, args ...any) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Options() *sql.TxOptions
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

// N6: Schema Qualification — modification of non-public table "archive.balances" is not in critical set.
func N6_ArchiveBalances(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE archive.balances SET amount = 100 WHERE id = 1")
	return tx.Commit(ctx)
}

// N7: String Literal Safety — query containing "balances" inside string literal is not a critical write.
func N7_AuditLogMessage(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "INSERT INTO audit_log (message) VALUES ('updated balances successfully')")
	return tx.Commit(ctx)
}

// N8: Correlated SQL Advisory Lock — Advisory lock explicitly correlates to "balances".
func N8_CorrelatedAdvisoryLock(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('balances:' || 1))")
	_ = tx.Exec(ctx, "UPDATE balances SET amount = amount - 100 WHERE id = 1")
	return tx.Commit(ctx)
}

// Calculator has Exec + Query but is not a database transaction.
type Calculator struct{}

func (Calculator) Exec(query string) error  { return nil }
func (Calculator) Query(query string) error { return nil }

// N9: Non-DB Calculator with Exec + Query must produce 0 violations.
func N9_NonDBCalculator() {
	calc := Calculator{}
	_ = calc.Exec("UPDATE balances SET amount = 0 WHERE id = 1")
}

// SearchEngine has Exec + Commit (no Rollback) and is not a database transaction.
type SearchEngine struct{}

func (SearchEngine) Exec(query string) error { return nil }
func (SearchEngine) Commit() error           { return nil }

// N10: Non-DB SearchEngine with Exec + Commit must produce 0 violations.
func N10_NonDBSearchEngine() {
	se := SearchEngine{}
	_ = se.Exec("UPDATE balances SET amount = 0 WHERE id = 1")
	_ = se.Commit()
}

// WorkerPool has Begin() but returns no transaction.
type WorkerPool struct{}

func (WorkerPool) Begin() {}

// N11: Non-DB WorkerPool with Begin() must produce 0 violations.
func N11_NonDBWorkerPool() {
	wp := WorkerPool{}
	wp.Begin()
}
