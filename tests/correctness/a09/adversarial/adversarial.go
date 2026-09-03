package adversarial

import (
	"context"
)

type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

type Helper struct{}

func (Helper) WithAdvisoryLock(ctx context.Context, tx any, lockName string, failFast bool, fn func() error) error {
	return fn()
}

// A1: Branch — conditional session-level advisory lock inside branch.
func A1_Branch(ctx context.Context, db DBExecutor, key int64, isLeader bool) error {
	if isLeader {
		_, err := db.Exec(ctx, "SELECT pg_advisory_lock($1)", key)
		return err
	}
	return nil
}

// A2: Reassignment — variable reassigned to session lock.
func A2_Reassignment(ctx context.Context, db DBExecutor, key int64) error {
	q := "SELECT pg_advisory_xact_lock($1)"
	_ = q
	q = "SELECT pg_advisory_lock($1)"
	_, err := db.Exec(ctx, q, key)
	return err
}

// A3: Alias — aliased query string with hardcoded int key.
func A3_Alias(ctx context.Context, db DBExecutor) error {
	raw := "SELECT pg_advisory_xact_lock(42)"
	alias := raw
	_, err := db.Exec(ctx, alias)
	return err
}

// A4: Wrapper — lock manager struct executing session-level unlock.
type LockMgr struct {
	db DBExecutor
}

func (l *LockMgr) Unlock(ctx context.Context) error {
	_, err := l.db.Exec(ctx, "SELECT pg_advisory_unlock(1)")
	return err
}

// A5: Nested Function — closure executing session-level try lock.
func A5_NestedFunction(ctx context.Context, db DBExecutor) error {
	tryLock := func() error {
		_, err := db.Exec(ctx, "SELECT pg_try_advisory_lock(1)")
		return err
	}
	return tryLock()
}

// A6: Generic — generic coordinator executing session unlock all.
type Coordinator[T any] struct {
	db DBExecutor
}

func (c *Coordinator[T]) Reset(ctx context.Context) error {
	_, err := c.db.Exec(ctx, "SELECT pg_advisory_unlock_all()")
	return err
}

// A7: Interface / Helper — empty lock name in helper call.
func A7_HelperEmptyName(ctx context.Context, tx any, h Helper) error {
	return h.WithAdvisoryLock(ctx, tx, "", false, func() error {
		return nil
	})
}
