package negative

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

// N1: Obvious Safe — parameterized transaction-scoped advisory lock.
func N1_ObviousSafe(ctx context.Context, db DBExecutor, key int64) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", key)
	return err
}

// N2: Legitimate Idiom — two-parameter classID/objID transaction-scoped lock.
func N2_LegitimateIdiom(ctx context.Context, db DBExecutor, classID, objID int32) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_xact_lock($1, $2)", classID, objID)
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

// N4: Sanitized Helper Call — valid namespaced identifier in WithAdvisoryLock helper.
func N4_SanitizedHelper(ctx context.Context, tx any) error {
	return argus.WithAdvisoryLock(ctx, tx, "payout:lock:123", true, func() error {
		return nil
	})
}

// N5: Static Standard DML — standard application query unrelated to locks.
func N5_StandardDML(ctx context.Context, db DBExecutor) error {
	_, err := db.Query(ctx, "SELECT id, name FROM users WHERE id = $1", "1")
	return err
}
