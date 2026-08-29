package a18

import (
	"context"
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

type PB struct{}

func (PB) Next() bool { return false }

type User struct {
	ID   int
	Name string
}

func CollectRows(rows Rows) ([]User, error) {
	return nil, nil
}

func Cases(ctx context.Context, db DB, pb PB) ([]User, error) {
	// 1. Missing rows.Err() check before return (Violation)
	rows, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users1 []User
	for rows.Next() { // want `\[ARGUS-A18\] missing rows\.Err\(\) check after for rows\.Next\(\) loop; unchecked error risks silent dataset truncation on network drop or statement timeout \(CWE-391\)`
		var u User
		_ = rows.Scan(&u.ID, &u.Name)
		users1 = append(users1, u)
	}

	// 2. Missing r.Err() check with custom variable name (Violation)
	r, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var users2 []User
	for r.Next() { // want `\[ARGUS-A18\] missing r\.Err\(\) check after for r\.Next\(\) loop; unchecked error risks silent dataset truncation on network drop or statement timeout \(CWE-391\)`
		var u User
		_ = r.Scan(&u.ID, &u.Name)
		users2 = append(users2, u)
	}

	// 3. Compliant if err := rows.Err(); err != nil
	rows3, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows3.Close()

	var users3 []User
	for rows3.Next() {
		var u User
		_ = rows3.Scan(&u.ID, &u.Name)
		users3 = append(users3, u)
	}
	if err := rows3.Err(); err != nil {
		return nil, err
	}

	// 4. Compliant if rows.Err() != nil
	rows4, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows4.Close()

	var users4 []User
	for rows4.Next() {
		var u User
		_ = rows4.Scan(&u.ID, &u.Name)
		users4 = append(users4, u)
	}
	if rows4.Err() != nil {
		return nil, rows4.Err()
	}

	// 5. Compliant err = rows.Err()
	rows5, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows5.Close()

	var users5 []User
	for rows5.Next() {
		var u User
		_ = rows5.Scan(&u.ID, &u.Name)
		users5 = append(users5, u)
	}
	err = rows5.Err()
	if err != nil {
		return nil, err
	}

	// 6. Compliant return users, rows.Err()
	rows6, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows6.Close()

	var users6 []User
	for rows6.Next() {
		var u User
		_ = rows6.Scan(&u.ID, &u.Name)
		users6 = append(users6, u)
	}

	// 7. Non-database pb.Next()
	for pb.Next() {
		// Non-database iterator
	}

	// 8. Ignored loop via full directive
	rows8, _ := db.Query(ctx, "SELECT 1")
	// argus:ignore ARGUS-A18 telemetry sampler allows best-effort partial stream
	for rows8.Next() {
	}

	// 9. Ignored loop via shortcode directive
	rows9, _ := db.Query(ctx, "SELECT 1")
	// argus:ignore-a18 best-effort log streamer
	for rows9.Next() {
	}

	return users6, rows6.Err()
}
