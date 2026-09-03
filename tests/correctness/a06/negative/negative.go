package negative

import (
	"context"
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
