package adversarial

import (
	"context"
)

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

// A1: Branch — conditional DDL execution inside branch.
func A1_Branch(ctx context.Context, db DBExecutor, shouldDrop bool) error {
	if shouldDrop {
		_, err := db.Exec(ctx, "DROP TABLE legacy_users")
		return err
	}
	return nil
}

// A2: Reassignment — variable initialized to safe DML, then reassigned to DDL.
func A2_Reassignment(ctx context.Context, db DBExecutor) error {
	q := "SELECT 1"
	_ = q
	q = "CREATE TABLE audit_shadow (id int)"
	_, err := db.Exec(ctx, q)
	return err
}

// A3: Alias — aliased query string variable.
func A3_Alias(ctx context.Context, db DBExecutor) error {
	raw := "GRANT ALL ON users TO app_user"
	alias := raw
	_, err := db.Exec(ctx, alias)
	return err
}

// A4: Wrapper — repository struct wrapping DB and executing CREATE INDEX.
type Migrator struct {
	db DBExecutor
}

func (m *Migrator) CreateIndex(ctx context.Context) error {
	_, err := m.db.Exec(ctx, "CREATE INDEX idx_users_email ON users (email)")
	return err
}

// A5: Nested Function — closure executing TRUNCATE.
func A5_NestedFunction(ctx context.Context, db DBExecutor) error {
	purge := func() error {
		_, err := db.Exec(ctx, "TRUNCATE TABLE temp_sessions")
		return err
	}
	return purge()
}

// A6: Generic — generic struct executing DROP SCHEMA.
type SchemaManager[T any] struct {
	db DBExecutor
}

func (s *SchemaManager[T]) DropSchema(ctx context.Context) error {
	_, err := s.db.Exec(ctx, "DROP SCHEMA test_schema CASCADE")
	return err
}

// A7: Interface — dynamic interface assertion before executing ALTER TABLE.
func A7_Interface(ctx context.Context, client any) error {
	if exec, ok := client.(DBExecutor); ok {
		_, err := exec.Exec(ctx, "ALTER TABLE accounts ADD COLUMN status text")
		return err
	}
	return nil
}

// A8: Variable Shadowing — inner scope executes shadowed DDL, outer scope executes safe query.
func A8_VariableShadowing(ctx context.Context, db DBExecutor) error {
	query := "SELECT 1"
	{
		query := "CREATE TABLE shadowed (id int)"
		if _, err := db.Exec(ctx, query); err != nil {
			return err
		}
	}
	_, err := db.Exec(ctx, query)
	return err
}

// A9: Branch Reassignment — query initialized to SELECT, conditionally reassigned to DDL (MAYBE_DDL must be caught).
func A9_BranchReassignment(ctx context.Context, db DBExecutor, cond bool) error {
	query := "SELECT 1"
	if cond {
		query = "CREATE TABLE branch_table (id int)"
	}
	_, err := db.Exec(ctx, query)
	return err
}

// A10: Non-DB Type Spoofing — Calculator type with Exec method should NOT be flagged as DB call.
type Calculator struct{}

func (c Calculator) Exec(ctx context.Context, q string) (any, error) {
	return nil, nil
}

func A10_NonDBTypeSpoofing(ctx context.Context) error {
	calc := Calculator{}
	_, err := calc.Exec(ctx, "CREATE TABLE spoofed (id int)")
	return err
}

// A11: Custom Builder Type — SQLBuilder type with "builder" in name should NOT be treated as strings.Builder.
type SQLBuilder struct{}

func (s *SQLBuilder) String() string {
	return "SELECT 1"
}

func A11_CustomBuilderTypeSpoofing(ctx context.Context, db DBExecutor) error {
	sb := SQLBuilder{}
	query := sb.String()
	_, err := db.Exec(ctx, query)
	return err
}

