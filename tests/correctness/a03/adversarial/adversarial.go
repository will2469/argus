package adversarial

import (
	"context"
	"time"
)

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	BeginTx(ctx context.Context, opts any) (any, error)
}

// A1: Branch — conditional branch introducing raw context.
func A1_Branch(db DBExecutor, fallback bool) error {
	if fallback {
		ctx := context.Background()
		_, err := db.Query(ctx, "SELECT id FROM users")
		return err
	}
	return nil
}

// A2: Reassignment — raw context overridden by bounded timeout (Clean / must survive!).
func A2_Reassignment_Clean(db DBExecutor) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := db.Query(ctx, "SELECT id FROM users")
	return err
}

// A3: Alias — raw context aliased through multiple assignments.
func A3_Alias(db DBExecutor) error {
	c1 := context.Background()
	c2 := c1
	c3 := c2
	_, err := db.Query(c3, "SELECT id FROM orders")
	return err
}

// A4: Wrapper — repository struct wrapping database connection.
type UserRepository struct {
	db DBExecutor
}

func (r *UserRepository) FindAll() error {
	_, err := r.db.Query(context.TODO(), "SELECT id FROM users")
	return err
}

// A5: Nested Function — closure executing query with unbounded context.
func A5_NestedFunction(db DBExecutor) {
	run := func() {
		_, _ = db.Query(context.Background(), "SELECT id FROM audit_logs")
	}
	run()
}

// A6: Generic — generic store calling query with raw context.
type GenericStore[T any] struct {
	db DBExecutor
}

func (s *GenericStore[T]) FetchAll() error {
	_, err := s.db.Query(context.Background(), "SELECT id FROM items")
	return err
}

// A7: Interface — dynamic interface assertion before executing query with raw context.
func A7_Interface(anyDB any) error {
	if exec, ok := anyDB.(DBExecutor); ok {
		_, err := exec.Query(context.Background(), "SELECT id FROM tenants")
		return err
	}
	return nil
}
