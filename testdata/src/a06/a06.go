package a06

import (
	"context"
	"fmt"
)

type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func SafeQuery(ctx context.Context, db DBExecutor) error {
	_, err := db.Query(ctx, "SELECT id, name FROM users WHERE id = $1", "1")
	return err
}

func SafeInsert(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "INSERT INTO users (id, name) VALUES ($1, $2)", "1", "Alice")
	return err
}

func BadCreateTable(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "CREATE TABLE temp_orders (id int)") // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(CREATE TABLE\)`
	return err
}

func BadDropTable(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "DROP TABLE legacy_users") // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(DROP\)`
	return err
}

func BadAlterTable(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "ALTER TABLE users ADD COLUMN bio text") // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(ALTER TABLE\)`
	return err
}

func BadTruncate(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "TRUNCATE TABLE cached_tokens") // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(TRUNCATE\)`
	return err
}

func BadGrant(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "GRANT ALL ON users TO app_user") // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(GRANT/REVOKE\)`
	return err
}

func BadDynamicCreate(ctx context.Context, db DBExecutor, suffix string) error {
	q := fmt.Sprintf("CREATE TABLE tenant_%s (id int)", suffix)
	_, err := db.Exec(ctx, q) // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(CREATE TABLE\)`
	return err
}

func BadMultiStatementDDL(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "SELECT 1; DROP TABLE users;") // want `\[ARGUS-A06\] runtime database query contains forbidden DDL statement \(DROP\)`
	return err
}

func IgnoredDDL(ctx context.Context, db DBExecutor) error {
	// argus:ignore ARGUS-A06 test runner ephemeral schema generation
	_, err := db.Exec(ctx, "CREATE TABLE ephemeral_test (id int)")
	return err
}
