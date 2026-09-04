package negative

import (
	"context"
	"fmt"
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

// N1: Obvious Safe — parameterized transaction-scoped advisory lock.
func N1_ObviousSafe(ctx context.Context, db DBExecutor, key int64) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key)
	return err
}

// N2: Legitimate Idiom — 2-parameter namespace constant classID + dynamic resource objID.
func N2_LegitimateIdiom(ctx context.Context, db DBExecutor, objID int32) error {
	const namespaceID = 1001
	_ = namespaceID
	_, err := db.Exec(ctx, "SELECT pg_advisory_xact_lock(1001, $1)", objID)
	return err
}

// N3: Unrelated API — non-database client method.
type CacheClient struct{}

func (CacheClient) Exec(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

func N3_UnrelatedAPI(ctx context.Context, cache CacheClient) error {
	_, err := cache.Exec(ctx, "SELECT pg_advisory_lock(1)")
	return err
}

// N4: Sanitized Helper Call — valid namespaced identifier with delimiter in WithAdvisoryLock helper.
func N4_SanitizedHelper(ctx context.Context, tx any) error {
	return argus.WithAdvisoryLock(ctx, tx, "payout:lock:123", true, func() error {
		return nil
	})
}

// N5: Non-SQL Argument Safety — string parameter containing SQL snippet does not trigger false positive.
func N5_ParameterSQLSafety(ctx context.Context, db DBExecutor) error {
	_, err := db.Query(ctx, "SELECT id, name FROM audit_logs WHERE message = $1", "SELECT pg_advisory_lock(1)")
	return err
}

// N6: 2-Arg Dynamic Columns — both arguments are dynamic query columns or expressions.
func N6_DynamicTwoArgs(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_xact_lock(namespace_id, resource_id) FROM tenant_locks")
	return err
}

// N7: Sprintf Helper Call — fmt.Sprintf with delimited namespace format.
func N7_SprintfHelper(ctx context.Context, tx any, orderID string) error {
	return argus.WithAdvisoryLock(ctx, tx, fmt.Sprintf("orders:%s", orderID), true, func() error {
		return nil
	})
}
