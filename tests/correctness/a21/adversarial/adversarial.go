package adversarial

import (
	"context"
)

type DB struct{}

func (DB) Query(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

func (DB) QueryRow(ctx context.Context, sql string, args ...any) any {
	return nil
}

// A1: Branch — conditional blocking lock inside branch.
func A1_Branch(ctx context.Context, db DB, cond bool) {
	if cond {
		_ = db.QueryRow(ctx, "SELECT id FROM queue WHERE status = 'WAITING' FOR UPDATE")
	}
}

// A2: Reassignment — query variable assigned across statements.
func A2_Reassignment(ctx context.Context, db DB) {
	q := "SELECT id FROM events FOR UPDATE"
	_ = db.QueryRow(ctx, q)
}

// A3: Alias — query string aliased to another variable.
func A3_Alias(ctx context.Context, db DB) {
	raw := "SELECT id FROM tasks WHERE ready = true FOR NO KEY UPDATE"
	alias := raw
	_, _ = db.Query(ctx, alias)
}

// A4: Wrapper — DAO struct method executing blocking lock.
type LockDAO struct {
	db DB
}

func (d LockDAO) Acquire(ctx context.Context) {
	_ = d.db.QueryRow(ctx, "SELECT id FROM locks WHERE name = 'sync' FOR UPDATE")
}

// A5: Nested Function — closure executing blocking lock.
func A5_NestedFunction(ctx context.Context, db DB) {
	worker := func() {
		_ = db.QueryRow(ctx, "SELECT id FROM items WHERE processing = false FOR UPDATE")
	}
	worker()
}

// A6: Generic — generic locking worker.
type Worker[T any] struct {
	db DB
}

func (w Worker[T]) Run(ctx context.Context) {
	_ = w.db.QueryRow(ctx, "SELECT id FROM work_units FOR UPDATE")
}

// A7: CTE — Common Table Expression containing blocking FOR UPDATE.
func A7_CTE(ctx context.Context, db DB) {
	const query = `
		WITH locked AS (
			SELECT id FROM tasks WHERE active = true FOR UPDATE
		)
		SELECT id FROM locked
	`
	_, _ = db.Query(ctx, query)
}
