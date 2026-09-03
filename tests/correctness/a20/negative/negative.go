package negative

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

// N1: Obvious Safe — Binary CopyFrom protocol is immune to wire parameter limits.
func N1_ObviousSafe(ctx context.Context, db DB, users []User) error {
	_, err := db.CopyFrom(ctx, "users", []string{"id", "name", "email"}, users)
	return err
}

// N2: Legitimate Idiom — PostgreSQL Array ANY($1) uses exactly one bind parameter.
func N2_LegitimateIdiom(ctx context.Context, db DB, ids []string) error {
	_, err := db.Query(ctx, "SELECT id, name FROM users WHERE id = ANY($1)", ids)
	return err
}

// N3: Safe Chunking — Dynamic VALUES guarded within bounded chunk loop.
func N3_SafeChunking(ctx context.Context, db DB, users []User) error {
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

// N4: Static Single Row — Standard static INSERT with fixed bind parameters.
func N4_StaticSingleRow(ctx context.Context, db DB) error {
	_, err := db.Exec(ctx, "INSERT INTO users (id, name) VALUES ($1, $2)", "1", "Alice")
	return err
}

// N5: Static Keyset Pagination — Bounded pagination using inequality and limit.
func N5_KeysetPagination(ctx context.Context, db DB, lastID string) error {
	_, err := db.Query(ctx, "SELECT id, name FROM users WHERE id > $1 ORDER BY id ASC LIMIT 100", lastID)
	return err
}
