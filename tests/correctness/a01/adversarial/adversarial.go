package adversarial

import (
	"context"
	"fmt"
)

// DBExecutor represents a standard database interface for queries.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	QueryRow(ctx context.Context, sql string, args ...any) any
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

// A1: Branch — conditional query construction across branching statements.
func A1_Branch(ctx context.Context, db DBExecutor, filter string, enabled bool) {
	q := "SELECT id FROM accounts"
	if enabled {
		q += " WHERE type = '" + filter + "'"
	}
	_, _ = db.Query(ctx, q)
}

// A2: Reassignment — dirty variable reassigned to clean, and clean reassigned to dirty.
func A2_Reassignment_CleanOverride(ctx context.Context, db DBExecutor, id string) {
	q := "SELECT * FROM users WHERE id = " + id
	_ = q
	// Overridden by compliant query string
	q = "SELECT * FROM users WHERE id = $1"
	_, _ = db.Query(ctx, q, id)
}

func A2_Reassignment_DirtyOverride(ctx context.Context, db DBExecutor, id string) {
	q := "SELECT * FROM users WHERE id = $1"
	_ = q
	// Overridden by non-compliant concatenated string
	q = "SELECT * FROM users WHERE id = " + id
	_, _ = db.Query(ctx, q)
}

// A3: Alias — query aliased through pointer indirection.
func A3_Alias(ctx context.Context, db DBExecutor, id string) {
	rawSQL := "SELECT * FROM users WHERE id = " + id
	ptr := &rawSQL
	_, _ = db.Query(ctx, *ptr)
}

// A4: Wrapper — user repository struct wrapping the database executor.
type AccountRepository struct {
	pool DBExecutor
}

func NewAccountRepository(pool DBExecutor) *AccountRepository {
	return &AccountRepository{pool: pool}
}

func (r *AccountRepository) FindByRawID(ctx context.Context, id string) {
	_, _ = r.pool.Query(ctx, "SELECT * FROM accounts WHERE id = "+id)
}

// A5: Nested Function — closure capturing outer variable and executing query.
func A5_NestedFunction(ctx context.Context, db DBExecutor, id string) {
	runClosure := func() {
		sql := fmt.Sprintf("DELETE FROM audit_logs WHERE id = '%s'", id)
		_, _ = db.Exec(ctx, sql)
	}
	runClosure()
}

// A6: Generic — generic repository struct with parameterized model type.
type GenericRepository[T any] struct {
	db DBExecutor
}

func (g *GenericRepository[T]) FindUnsafe(ctx context.Context, id string) {
	_, _ = g.db.Query(ctx, "SELECT * FROM items WHERE id = "+id)
}

// A7: Interface — dynamic interface type assertion before query execution.
func A7_Interface(ctx context.Context, anyClient any, id string) {
	if exec, ok := anyClient.(DBExecutor); ok {
		_, _ = exec.Query(ctx, "SELECT * FROM tenants WHERE id = "+id)
	}
}
