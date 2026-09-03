package negative

import (
	"context"
	_ "database/sql"
)

type DB struct{}

type Rows struct{}

func (Rows) Next() bool             { return false }
func (Rows) Scan(dest ...any) error { return nil }
func (Rows) Err() error             { return nil }
func (Rows) Close()                 {}

func (DB) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return Rows{}, nil
}

type User struct {
	ID   int
	Name string
}

// N1: Obvious Safe — checked via `if err := rows.Err(); err != nil`.
func N1_ObviousSafe(ctx context.Context, db DB) ([]User, error) {
	rows, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		_ = rows.Scan(&u.ID, &u.Name)
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

// N2: Legitimate Idiom — checked via `if rows.Err() != nil`.
func N2_LegitimateIdiom(ctx context.Context, db DB) ([]User, error) {
	rows, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		_ = rows.Scan(&u.ID, &u.Name)
		users = append(users, u)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return users, nil
}

// N3: Unrelated API — non-database iterator pb.Next().
type PB struct{}

func (PB) Next() bool { return false }

func N3_UnrelatedAPI(pb PB) {
	for pb.Next() {
	}
}

// N4: Sanitized Input — assignment `err = rows.Err()` followed by nil check.
func N4_AssignedErrCheck(ctx context.Context, db DB) error {
	rows, err := db.Query(ctx, "SELECT 1")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
	}
	err = rows.Err()
	if err != nil {
		return err
	}
	return nil
}

// N5: Direct Return — returning `rows.Err()` directly in return statement.
func N5_DirectReturn(ctx context.Context, db DB) ([]User, error) {
	rows, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		_ = rows.Scan(&u.ID, &u.Name)
		users = append(users, u)
	}
	return users, rows.Err()
}
