package adversarial

import (
	"context"
	_ "database/sql"
)

type DB struct{}

type Rows struct{}

func (Rows) Next() bool             { return false }
func (Rows) Scan(dest ...any) error { return nil }
func (Rows) Err() error             { return nil }
func (Rows) Close()                 {}

func (DB) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return Rows{}, nil
}

// A1: Branch — cursor loop inside branch missing Err().
func A1_Branch(ctx context.Context, db DB, cond bool) error {
	if cond {
		rows, _ := db.Query(ctx, "SELECT 1")
		for rows.Next() {
		}
	}
	return nil
}

// A2: Reassignment — cursor variable reassigned before loop.
func A2_Reassignment(ctx context.Context, db DB) error {
	rows, _ := db.Query(ctx, "SELECT 1")
	cursor := rows
	for cursor.Next() {
	}
	return nil
}

// A3: Alias — single-letter variable loop without Err().
func A3_Alias(r Rows) {
	for r.Next() {
	}
}

// A4: Wrapper — struct method wrapping query loop without Err().
type Repo struct {
	db DB
}

func (repo Repo) Stream(ctx context.Context) {
	rows, _ := repo.db.Query(ctx, "SELECT 1")
	for rows.Next() {
	}
}

// A5: Nested Function — closure executing cursor loop without Err().
func A5_NestedFunction(rows Rows) {
	process := func() {
		for rows.Next() {
		}
	}
	process()
}

// A6: Generic — generic method with cursor loop without Err().
type Service[T any] struct {
	db DB
}

func (s Service[T]) Load(ctx context.Context) {
	rows, _ := s.db.Query(ctx, "SELECT 1")
	for rows.Next() {
	}
}

// A7: Wrong Variable Checked — developer looped over rows, but checked wrong cursor.
func A7_WrongVarCheck(ctx context.Context, db DB, otherRows Rows) error {
	rows, _ := db.Query(ctx, "SELECT 1")
	for rows.Next() {
	}
	if err := otherRows.Err(); err != nil {
		return err
	}
	return nil
}
