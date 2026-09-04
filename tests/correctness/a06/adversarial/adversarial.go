package adversarial

import (
	"context"
	"database/sql"
)

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (sql.Result, error)
	Query(ctx context.Context, sql string, args ...any) (*sql.Rows, error)
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

// A10: Non-DB Type Spoofing — SearchEngine type with Query AND Exec methods should NOT be flagged as DB call.
type SearchEngine interface {
	Query(ctx context.Context, q string) (any, error)
	Exec(ctx context.Context, q string) (any, error)
}

func A10_NonDBTypeSpoofing(ctx context.Context, engine SearchEngine) error {
	_, err := engine.Exec(ctx, "CREATE TABLE spoofed (id int)")
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

// A12: Unconventional Receiver Name — receiver named client should still be proven via DBExecutor type.
func A12_UnconventionalReceiverName(ctx context.Context, client DBExecutor) error {
	_, err := client.Exec(ctx, "CREATE TABLE custom_receiver (id int)")
	return err
}

// A13: Fake DB Receiver Name — receiver named db but with non-DB SearchEngine type must NOT be flagged.
func A13_FakeDBReceiverName(ctx context.Context, db SearchEngine) error {
	_, err := db.Exec(ctx, "CREATE TABLE fake_db (id int)")
	return err
}

// A14: Fake DB with Evil Implementation — FakeDB interface implemented by Evil struct returning nil must NOT be flagged.
type FakeDB interface {
	Exec(cmd string) (sql.Result, error)
	Query(cmd string) (*sql.Rows, error)
}

type Evil struct{}

func (Evil) Exec(cmd string) (sql.Result, error) {
	return nil, nil
}

func (Evil) Query(cmd string) (*sql.Rows, error) {
	return nil, nil
}

func A14_EvilImplementation_MustBeSafe(e Evil, db FakeDB) {
	_, _ = e.Exec("CREATE TABLE evil_table (id int)")
	_, _ = db.Exec("CREATE TABLE fake_table (id int)")
}

