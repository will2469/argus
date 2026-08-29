package a09

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

var argus Helper

func SafeXactLock(ctx context.Context, db DBExecutor, key int64) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key)
	return err
}

func SafeTwoArgXactLock(ctx context.Context, db DBExecutor, classID, objID int32) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_xact_lock($1, $2)", classID, objID)
	return err
}

func SafeHelperCall(ctx context.Context, tx any) error {
	return argus.WithAdvisoryLock(ctx, tx, "payout:lock:123", true, func() error {
		return nil
	})
}

func BadSessionLock(ctx context.Context, db DBExecutor, key int64) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_lock($1)", key) // want `\[ARGUS-A09\] forbidden session-level advisory lock "pg_advisory_lock"`
	return err
}

func BadHardcodedKey(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_xact_lock(1)") // want `\[ARGUS-A09\] hardcoded integer advisory lock key in SQL`
	return err
}

func BadEmptyHelperName(ctx context.Context, tx any) error {
	return argus.WithAdvisoryLock(ctx, tx, "", true, func() error { // want `\[ARGUS-A09\] empty advisory lock name`
		return nil
	})
}

func IgnoredAdvisoryLock(ctx context.Context, db DBExecutor) error {
	// argus:ignore ARGUS-A09 dedicated non-pooled connection leader election
	_, err := db.Exec(ctx, "SELECT pg_advisory_lock($1)", 999)
	return err
}
