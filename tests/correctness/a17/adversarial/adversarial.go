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

// A1: Branch — conditional query inside loop.
func A1_Branch(ctx context.Context, db DB, ids []int) {
	for _, id := range ids {
		if id > 0 {
			_ = db.QueryRow(ctx, "SELECT 1 WHERE id = $1", id)
		}
	}
}

// A2: Reassignment — DB variable reassigned before loop.
func A2_Reassignment(ctx context.Context, db DB, ids []int) {
	d := db
	for _, id := range ids {
		_ = d.QueryRow(ctx, "SELECT 1 WHERE id = $1", id)
	}
}

// A3: Alias — DB pointer alias used in loop.
func A3_Alias(ctx context.Context, db DB, ids []int) {
	ptr := &db
	for _, id := range ids {
		_ = ptr.QueryRow(ctx, "SELECT 1 WHERE id = $1", id)
	}
}

// A4: Wrapper — repository struct method with internal query loop.
type Repo struct {
	db DB
}

func (r Repo) BatchGet(ctx context.Context, ids []int) {
	for _, id := range ids {
		_ = r.db.QueryRow(ctx, "SELECT 1", id)
	}
}

// A5: Nested Function — closure executed inside loop.
func A5_NestedFunction(ctx context.Context, db DB, ids []int) {
	for _, id := range ids {
		func() {
			_ = db.QueryRow(ctx, "SELECT 1", id)
		}()
	}
}

// A6: Generic — generic struct processor executing query in loop.
type Processor[T any] struct {
	db DB
}

func (p Processor[T]) Process(ctx context.Context, items []T) {
	for range items {
		_ = p.db.QueryRow(ctx, "SELECT 1")
	}
}

// A7: While Loop — while-style condition loop with query.
func A7_WhileLoop(ctx context.Context, db DB, ids []int) {
	i := 0
	for i < len(ids) {
		_ = db.QueryRow(ctx, "SELECT 1", ids[i])
		i++
	}
}
