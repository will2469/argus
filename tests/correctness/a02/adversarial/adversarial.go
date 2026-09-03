package adversarial

import (
	"context"
)

// Rows represents an active database cursor requiring explicit close.
type Rows interface {
	Close()
	Next() bool
}

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

// A1: Branch — query inside conditional branch missing defer close.
func A1_Branch(ctx context.Context, db DBExecutor, enabled bool) error {
	if enabled {
		rows, err := db.Query(ctx, "SELECT id FROM accounts")
		if err != nil {
			return err
		}
		_ = rows
	}
	return nil
}

// A2: Reassignment — rows variable reassigned without closing second query.
func A2_Reassignment(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	defer rows.Close()

	// Reassigned to second query without dedicated defer
	rows, err = db.Query(ctx, "SELECT id FROM orders")
	if err != nil {
		return err
	}
	_ = rows
	return nil
}

// A3: Alias — rows cursor assigned to an alias and safely closed via alias.
func A3_Alias_Clean(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	cursor := rows
	defer cursor.Close()
	return nil
}

// A4: Wrapper — repository struct wrapping database connection.
type UserRepository struct {
	db DBExecutor
}

func (r *UserRepository) FindAll(ctx context.Context) error {
	rows, err := r.db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	_ = rows
	return nil
}

// A5: Nested Function — query inside nested closure without defer.
func A5_NestedFunction(ctx context.Context, db DBExecutor) {
	run := func() {
		rows, err := db.Query(ctx, "SELECT id FROM audit_logs")
		if err != nil {
			return
		}
		_ = rows
	}
	run()
}

// A6: Generic — generic store calling Query without defer.
type GenericStore[T any] struct {
	db DBExecutor
}

func (s *GenericStore[T]) FetchAll(ctx context.Context) error {
	rows, err := s.db.Query(ctx, "SELECT id FROM items")
	if err != nil {
		return err
	}
	_ = rows
	return nil
}

// A7: Interface — dynamic interface assertion before executing unclosed query.
func A7_Interface(ctx context.Context, anyDB any) error {
	if exec, ok := anyDB.(DBExecutor); ok {
		rows, err := exec.Query(ctx, "SELECT id FROM tenants")
		if err != nil {
			return err
		}
		_ = rows
	}
	return nil
}
