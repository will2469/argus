package adversarial

import (
	"context"
)

type DB struct{}

func (DB) Query(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

// A1: Branch — conditional unbounded query inside branch.
func A1_Branch(ctx context.Context, db DB, filter bool) {
	if filter {
		_, _ = db.Query(ctx, "SELECT id FROM audit_logs WHERE action = 'LOGIN'")
	}
}

// A2: Reassignment — query variable without LIMIT.
func A2_Reassignment(ctx context.Context, db DB) {
	q := "SELECT id, payload FROM audit_logs"
	_, _ = db.Query(ctx, q)
}

// A3: Alias — query aliased to another variable.
func A3_Alias(ctx context.Context, db DB) {
	raw := "SELECT id, amount FROM transactions"
	alias := raw
	_, _ = db.Query(ctx, alias)
}

// A4: Wrapper — DAO struct method executing unbounded query.
type DAO struct {
	db DB
}

func (d DAO) List(ctx context.Context) {
	_, _ = d.db.Query(ctx, "SELECT id, status FROM orders")
}

// A5: Nested Function — closure executing unbounded query.
func A5_NestedFunction(ctx context.Context, db DB) {
	fn := func() {
		_, _ = db.Query(ctx, "SELECT id, type FROM events")
	}
	fn()
}

// A6: Generic — generic service executing unbounded query.
type Svc[T any] struct {
	db DB
}

func (s Svc[T]) Fetch(ctx context.Context) {
	_, _ = s.db.Query(ctx, "SELECT id, msg FROM activity_logs")
}

// A7: Schema Prefix — schema qualified high-cardinality table.
func A7_SchemaPrefix(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id FROM public.audit_logs")
}
