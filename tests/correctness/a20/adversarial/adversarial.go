package adversarial

import (
	"context"
	"fmt"
	"strings"
)

type DB struct{}

func (DB) Exec(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

func (DB) Query(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

// A1: Branch — conditional dynamic IN clause without chunking.
func A1_Branch(ctx context.Context, db DB, ids []string, cond bool) {
	if cond {
		var ph []string
		var args []any
		for i, id := range ids {
			ph = append(ph, fmt.Sprintf("$%d", i+1))
			args = append(args, id)
		}
		q := "SELECT id FROM items WHERE id IN (" + strings.Join(ph, ",") + ")"
		_, _ = db.Query(ctx, q, args...)
	}
}

// A2: Reassignment — query variable assigned in steps without chunking.
func A2_Reassignment(ctx context.Context, db DB, names []string) {
	var ph []string
	var args []any
	for i, n := range names {
		ph = append(ph, fmt.Sprintf("($%d)", i+1))
		args = append(args, n)
	}
	q := "INSERT INTO tags (name) VALUES "
	q = q + strings.Join(ph, ",")
	_, _ = db.Exec(ctx, q, args...)
}

// A3: Alias — slice aliased and iterated for values.
func A3_Alias(ctx context.Context, db DB, raw []string) {
	list := raw
	var ph []string
	var args []any
	for i, item := range list {
		ph = append(ph, fmt.Sprintf("($%d)", i+1))
		args = append(args, item)
	}
	stmt := "INSERT INTO queue (msg) VALUES " + strings.Join(ph, ",")
	_, _ = db.Exec(ctx, stmt, args...)
}

// A4: Wrapper — struct method performing unchunked batch insert.
type Batcher struct {
	db DB
}

func (b Batcher) InsertBatch(ctx context.Context, data []string) {
	var ph []string
	var args []any
	for i, d := range data {
		ph = append(ph, fmt.Sprintf("($%d)", i+1))
		args = append(args, d)
	}
	q := "INSERT INTO logs (data) VALUES " + strings.Join(ph, ",")
	_, _ = b.db.Exec(ctx, q, args...)
}

// A5: Nested Function — closure building dynamic batch.
func A5_NestedFunction(ctx context.Context, db DB, ids []string) {
	execute := func() {
		var ph []string
		var args []any
		for i, id := range ids {
			ph = append(ph, fmt.Sprintf("$%d", i+1))
			args = append(args, id)
		}
		q := "SELECT id FROM records WHERE id IN (" + strings.Join(ph, ",") + ")"
		_, _ = db.Query(ctx, q, args...)
	}
	execute()
}

// A6: Generic — generic batch processor with unchunked VALUES.
type GenericStore[T any] struct {
	db DB
}

func (s GenericStore[T]) BulkInsert(ctx context.Context, items []string) {
	var ph []string
	var args []any
	for i, it := range items {
		ph = append(ph, fmt.Sprintf("($%d)", i+1))
		args = append(args, it)
	}
	q := "INSERT INTO generic_items (val) VALUES " + strings.Join(ph, ",")
	_, _ = s.db.Exec(ctx, q, args...)
}

// A7: Strings Repeat — IN clause generation via strings.Repeat.
func A7_StringsRepeat(ctx context.Context, db DB, count int, args []any) {
	ph := strings.Repeat("?,", count)
	if len(ph) > 0 {
		ph = ph[:len(ph)-1]
	}
	q := "SELECT id FROM users WHERE id IN (" + ph + ")"
	_, _ = db.Query(ctx, q, args...)
}
