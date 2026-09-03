package adversarial

import (
	"context"
	"fmt"
)

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

// A1: Branch — conditional fallback to unvalidated user sort column.
func A1_Branch(ctx context.Context, db DBExecutor, userSort string, useFallback bool) error {
	var col string
	if useFallback {
		col = userSort
	} else {
		col = "id"
	}

	q := fmt.Sprintf("SELECT id FROM users ORDER BY %s ASC", col)
	_, err := db.Query(ctx, q)
	return err
}

// A2: Reassignment — safe initial value overridden by unvalidated user input.
func A2_Reassignment(ctx context.Context, db DBExecutor, userSort string) error {
	col := "id"
	if userSort != "" {
		col = userSort
	}

	q := fmt.Sprintf("SELECT id FROM users ORDER BY %s ASC", col)
	_, err := db.Query(ctx, q)
	return err
}

// A3: Alias — raw input aliased through variable indirection.
func A3_Alias(ctx context.Context, db DBExecutor, userSort string) error {
	raw := userSort
	alias := raw

	q := fmt.Sprintf("SELECT id FROM users ORDER BY %s DESC", alias)
	_, err := db.Query(ctx, q)
	return err
}

// A4: Wrapper — repository struct method building dynamic order by.
type AccountRepository struct {
	db DBExecutor
}

func (r *AccountRepository) List(ctx context.Context, sort string) error {
	q := fmt.Sprintf("SELECT id FROM accounts ORDER BY %s ASC", sort)
	_, err := r.db.Query(ctx, q)
	return err
}

// A5: Nested Function — closure constructing dynamic order by with raw input.
func A5_NestedFunction(ctx context.Context, db DBExecutor, userSort string) error {
	build := func() string {
		return fmt.Sprintf("SELECT id FROM audit_logs ORDER BY %s", userSort)
	}

	_, err := db.Query(ctx, build())
	return err
}

// A6: Generic — generic sorter interpolating unvalidated column name.
type Sorter[T any] struct {
	db DBExecutor
}

func (s *Sorter[T]) QuerySorted(ctx context.Context, sortCol string) error {
	q := fmt.Sprintf("SELECT id FROM items ORDER BY %s ASC", sortCol)
	_, err := s.db.Query(ctx, q)
	return err
}

// A7: Interface — interface returning unvalidated sort string.
type SortProvider interface {
	GetSortColumn() string
}

func A7_Interface(ctx context.Context, db DBExecutor, p SortProvider) error {
	q := fmt.Sprintf("SELECT id FROM records ORDER BY %s ASC", p.GetSortColumn())
	_, err := db.Query(ctx, q)
	return err
}
