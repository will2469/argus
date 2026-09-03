package positive

import (
	"context"
	"fmt"
)

// DBExecutor represents a standard database interface for queries.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

// P1: Obvious Violation — direct inline CREATE TABLE statement.
func P1_Obvious(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "CREATE TABLE temp_orders (id int)") // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(CREATE TABLE\)`
	return err
}

// P2: Indirect Violation — ALTER TABLE statement assembled into local variable.
func P2_Indirect(ctx context.Context, db DBExecutor) error {
	query := "ALTER TABLE users ADD COLUMN bio text"
	_, err := db.Exec(ctx, query) // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(ALTER TABLE\)`
	return err
}

// P3: Helper Violation — TRUNCATE on table inside internal helper subroutine.
func P3_Helper(ctx context.Context, db DBExecutor) error {
	return executeTruncateHelper(ctx, db)
}

func executeTruncateHelper(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "TRUNCATE TABLE cached_tokens") // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(TRUNCATE\)`
	return err
}

// P4: Nested Violation — multi-statement query containing DROP statement.
func P4_Nested(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "SELECT 1; DROP TABLE users;") // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(DROP\)`
	return err
}

// P5: Alias Violation — dynamic formatted query constructing CREATE TABLE with fmt.Sprintf.
func P5_Alias(ctx context.Context, db DBExecutor, suffix string) error {
	q := fmt.Sprintf("CREATE TABLE tenant_%s (id int)", suffix)
	_, err := db.Exec(ctx, q) // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(CREATE TABLE\)`
	return err
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, db DBExecutor) error {
	// argus:ignore ARGUS-A06 test runner ephemeral schema generation
	_, err := db.Exec(ctx, "CREATE TABLE ephemeral_test (id int)")
	return err
}
