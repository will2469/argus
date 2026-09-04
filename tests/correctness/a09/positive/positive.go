package positive

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

// P1: Obvious Violation — forbidden session-level advisory lock function.
func P1_Obvious(ctx context.Context, db DBExecutor, key int64) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_lock($1)", key) // want `\[ARGUS-A09\] forbidden session-level advisory lock "pg_advisory_lock"`
	return err
}

// P2: Indirect Violation — forbidden session-level shared lock assembled in variable.
func P2_Indirect(ctx context.Context, db DBExecutor, key int64) error {
	query := "SELECT pg_advisory_lock_shared($1)"
	_, err := db.Exec(ctx, query, key) // want `\[ARGUS-A09\] forbidden session-level advisory lock "pg_advisory_lock_shared"`
	return err
}

// P3: Helper Violation — hardcoded integer magic number lock key in 1-arg xact lock.
func P3_Helper(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_xact_lock(1)") // want `\[ARGUS-A09\] hardcoded integer advisory lock key in SQL`
	return err
}

// P4: Nested Violation — empty advisory lock name string in helper call.
func P4_Nested(ctx context.Context, tx any) error {
	return argus.WithAdvisoryLock(ctx, tx, "", true, func() error { // want `\[ARGUS-A09\] empty advisory lock name`
		return nil
	})
}

// P5: Alias Violation — session lock function inside multi-statement query.
func P5_Alias(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "SELECT 1; SELECT pg_try_advisory_lock(999)") // want `\[ARGUS-A09\] forbidden session-level advisory lock "pg_try_advisory_lock"`
	return err
}

// P6: Bare String Violation — unnamespaced bare string in WithAdvisoryLock helper.
func P6_BareHelperString(ctx context.Context, tx any) error {
	return argus.WithAdvisoryLock(ctx, tx, "foo", true, func() error { // want `\[ARGUS-A09\] unnamespaced advisory lock identifier "foo"`
		return nil
	})
}

// P7: Hardcoded Resource Key Violation — 2-arg advisory lock where resource ID is a hardcoded magic number.
func P7_HardcodedResourceKey(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_xact_lock(1001, 42)") // want `\[ARGUS-A09\] hardcoded integer advisory lock key in SQL`
	return err
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, db DBExecutor) error {
	// argus:ignore ARGUS-A09 dedicated non-pooled connection leader election
	_, err := db.Exec(ctx, "SELECT pg_advisory_lock($1)", 999)
	return err
}
