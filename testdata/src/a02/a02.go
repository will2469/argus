package a02

import (
	"context"
)

type Rows interface {
	Close()
	Next() bool
}

type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

type CollectHelper struct{}

func (CollectHelper) CollectRows(rows Rows) error {
	rows.Close()
	return nil
}

var pgx CollectHelper

func SafeDefer(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	defer rows.Close()
	return nil
}

func SafeDeferClosure(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	defer func() {
		rows.Close()
	}()
	return nil
}

func SafeAutoClosing(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	return pgx.CollectRows(rows)
}

func BadMissingDefer(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users") // want `\[ARGUS-A02\] missing defer rows\.Close\(\) for Query\(\) call`
	if err != nil {
		return err
	}
	_ = rows
	return nil
}

func BadNakedClose(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users") // want `\[ARGUS-A02\] missing defer rows\.Close\(\) for Query\(\) call`
	if err != nil {
		return err
	}
	rows.Close() // non-deferred close is prohibited
	return nil
}

func BadReturnRows(ctx context.Context, db DBExecutor) (Rows, error) {
	rows, err := db.Query(ctx, "SELECT id FROM users") // want `\[ARGUS-A02\] returning rows variable "rows" transfers resource ownership`
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func BadDirectReturnRows(ctx context.Context, db DBExecutor) (Rows, error) {
	return db.Query(ctx, "SELECT id FROM users") // want `\[ARGUS-A02\] returning rows transfers resource ownership`
}

func IgnoredMissingDefer(ctx context.Context, db DBExecutor) error {
	// argus:ignore ARGUS-A02 intentional mock generator in test fixture
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	_ = rows
	return nil
}
