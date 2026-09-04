package adversarial

import (
	"context"
	"database/sql"
)

type DB struct{ *sql.DB }

func (DB) Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error) {
	return nil, nil
}
// A1: Branch — conditional SELECT * inside branch.
func A1_Branch(ctx context.Context, db DB, debug bool) {
	if debug {
		_, _ = db.Query(ctx, "SELECT * FROM debug_table")
	}
}

// A2: Reassignment — query variable reassigned to SELECT *.
func A2_Reassignment(ctx context.Context, db DB) {
	query := "SELECT id FROM users"
	_ = query
	query = "SELECT * FROM users"
	_, _ = db.Query(ctx, query)
}

// A3: Alias — query string aliased through multiple variables.
func A3_Alias(ctx context.Context, db DB) {
	raw := "SELECT u.* FROM users u"
	alias := raw
	_, _ = db.Query(ctx, alias)
}

// A4: Wrapper — repository wrapper executing SELECT *.
type UserRepo struct {
	db DB
}

func (r UserRepo) GetAll(ctx context.Context) {
	_, _ = r.db.Query(ctx, "SELECT * FROM items")
}

// A5: Nested Function — closure executing SELECT *.
func A5_NestedFunction(ctx context.Context, db DB) {
	fetch := func() {
		_, _ = db.Query(ctx, "SELECT * FROM nested_data")
	}
	fetch()
}

// A6: Generic — generic service querying with SELECT *.
type Service[T any] struct {
	db DB
}

func (s Service[T]) Fetch(ctx context.Context) {
	_, _ = s.db.Query(ctx, "SELECT * FROM generic_table")
}

// A7: Interface — interface method call executing SELECT *.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error)
}

func A7_Interface(ctx context.Context, q Querier) {
	_, _ = q.Query(ctx, "SELECT * FROM dynamic_table")
}

// A8: Unconventional DB Variable Name — real DB receiver assigned to variable named calculator.
func A8_UnconventionalDBVarName(ctx context.Context, calculator DB) {
	_, _ = calculator.Query(ctx, "SELECT * FROM unconventional_table")
}
