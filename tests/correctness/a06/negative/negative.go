package negative

import (
	"context"
	"fmt"
)

// DBExecutor represents a standard database interface for queries.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

// N1: Obvious Safe — standard DML SELECT query.
func N1_ObviousSafe(ctx context.Context, db DBExecutor) error {
	_, err := db.Query(ctx, "SELECT id, name FROM users WHERE id = $1", "1")
	return err
}

// N2: Legitimate Idiom — standard DML INSERT query.
func N2_LegitimateIdiom(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "INSERT INTO users (id, name) VALUES ($1, $2)", "1", "Alice")
	return err
}

// N3: Unrelated API — non-database client with Exec method (e.g. Logger).
type Logger struct{}

func (Logger) Exec(ctx context.Context, msg string) (any, error) {
	return nil, nil
}

func N3_UnrelatedAPI(ctx context.Context, log Logger) error {
	_, err := log.Exec(ctx, "DROP TABLE key")
	return err
}

// N4: Standard DML Update — UPDATE query on application tables is compliant.
func N4_StandardUpdate(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "UPDATE users SET name = $1 WHERE id = $2", "Bob", 2)
	return err
}

// N5: Static Constant Input — standard DML DELETE query.
const CleanupQuery = "DELETE FROM users WHERE active = false"

func N5_StaticConstant(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, CleanupQuery)
	return err
}

// N6: Parameter Data Value — parameter value contains DDL string, but is bound parameter ($1) not SQL query.
func N6_ParameterDataValue(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "SELECT * FROM users WHERE name = $1", "CREATE TABLE totally_legit")
	return err
}

// N7: Dynamic Parameter Data Value — parameter value constructed via fmt.Sprintf is not an SQL query.
func N7_DynamicParameterValue(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "UPDATE audit_log SET payload = $1 WHERE id = $2", fmt.Sprintf("DROP TABLE %s", "fake"), 1)
	return err
}

// N8: Clean Reassignment — query variable initially assigned DDL is cleanly reassigned to DML before execution.
func N8_CleanReassignment(ctx context.Context, db DBExecutor, table string) error {
	q := "DROP TABLE " + table
	_ = q
	q = "SELECT id FROM users"
	_, err := db.Exec(ctx, q)
	return err
}

// N9: Dynamic DML with DDL Word — dynamic concatenation of DML with DDL keyword in value.
func N9_DynamicDMLWithDDLWord(ctx context.Context, db DBExecutor, table string) error {
	query := "SELECT * FROM " + table + " WHERE note = 'CREATE TABLE'"
	_, err := db.Exec(ctx, query)
	return err
}

type Calculator struct{}

func (*Calculator) WriteString(s string) {}
func (*Calculator) String() string       { return "" }

// N10: Unrelated WriteString — calculator WriteString does not poison SQL query buffer.
func N10_UnrelatedWriteString(ctx context.Context, db DBExecutor) error {
	var calc Calculator
	calc.WriteString("CREATE TABLE users (id int)")
	_, err := db.Exec(ctx, "SELECT id FROM users")
	return err
}

