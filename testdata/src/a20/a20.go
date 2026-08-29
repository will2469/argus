package a20

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

func (DB) CopyFrom(ctx context.Context, table any, columns []string, src any) (int64, error) {
	return 0, nil
}

type User struct {
	ID    string
	Name  string
	Email string
}

// 1. Safe pgx.CopyFrom (Compliant)
func SafeCopyFrom(ctx context.Context, db DB, users []User) error {
	_, err := db.CopyFrom(ctx, "users", []string{"id", "name", "email"}, users)
	return err
}

// 2. Safe PostgreSQL Array ANY($1) (Compliant)
func SafeAnyOperator(ctx context.Context, db DB, ids []string) error {
	_, err := db.Query(ctx, "SELECT id, name FROM users WHERE id = ANY($1)", ids)
	return err
}

// 3. Unsafe dynamic multi-row VALUES without chunking (Violation)
func UnsafeDynamicValues(ctx context.Context, db DB, users []User) error {
	var placeholders []string
	var args []any

	for i, u := range users {
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d)", i*3+1, i*3+2, i*3+3))
		args = append(args, u.ID, u.Name, u.Email)
	}

	query := "INSERT INTO users (id, name, email) VALUES " + strings.Join(placeholders, ",")
	_, err := db.Exec(ctx, query, args...) // want `\[ARGUS-A20\] unbounded dynamic multi-row VALUES batch construction without chunking; risk of exceeding 65,535 bind parameter limit; recommend pgx.CopyFrom \(CWE-400\)`
	return err
}

// 4. Safe dynamic multi-row VALUES with chunking loop (Compliant)
func SafeChunkedValues(ctx context.Context, db DB, users []User) error {
	const chunkSize = 500

	for i := 0; i < len(users); i += chunkSize {
		end := i + chunkSize
		if end > len(users) {
			end = len(users)
		}
		chunk := users[i:end]

		var placeholders []string
		var args []any
		for j, u := range chunk {
			placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d)", j*3+1, j*3+2, j*3+3))
			args = append(args, u.ID, u.Name, u.Email)
		}

		query := "INSERT INTO users (id, name, email) VALUES " + strings.Join(placeholders, ",")
		if _, err := db.Exec(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}

// 5. Unsafe dynamic IN clause placeholder generation (Violation)
func UnsafeDynamicInClause(ctx context.Context, db DB, ids []string) error {
	var placeholders []string
	var args []any
	for i, id := range ids {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
		args = append(args, id)
	}

	query := fmt.Sprintf("SELECT id, name FROM users WHERE id IN (%s)", strings.Join(placeholders, ","))
	_, err := db.Query(ctx, query, args...) // want `\[ARGUS-A20\] unbounded dynamic IN clause placeholder generation; risk of exceeding 65,535 bind parameter limit; recommend 'WHERE col = ANY\(\$1\)' \(CWE-400\)`
	return err
}

// 6. Ignored dynamic query via directive
func IgnoredDynamicValues(ctx context.Context, db DB, users []User) error {
	var placeholders []string
	var args []any
	for i, u := range users {
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		args = append(args, u.ID, u.Name)
	}
	query := "INSERT INTO users (id, name) VALUES " + strings.Join(placeholders, ",")

	// argus:ignore ARGUS-A20 bounded test dataset guaranteed under 500 parameters
	_, err := db.Exec(ctx, query, args...)
	return err
}

// 7. Ignored dynamic query via shortcode directive
func IgnoredShortcode(ctx context.Context, db DB, users []User) error {
	var placeholders []string
	var args []any
	for i, u := range users {
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		args = append(args, u.ID, u.Name)
	}
	query := "INSERT INTO users (id, name) VALUES " + strings.Join(placeholders, ",")

	// argus:ignore-a20 legacy migration batch
	_, err := db.Exec(ctx, query, args...)
	return err
}
