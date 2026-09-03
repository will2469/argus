package positive

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

// P1: Obvious Violation — Standard rows.Next() loop missing rows.Err() check.
func P1_Obvious(ctx context.Context, db DB) ([]User, error) {
	rows, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() { // want `\[ARGUS-A18\] missing rows\.Err\(\) check after for rows\.Next\(\) loop; unchecked error risks silent dataset truncation on network drop or statement timeout \(CWE-391\)`
		var u User
		_ = rows.Scan(&u.ID, &u.Name)
		users = append(users, u)
	}
	return users, nil
}

// P2: Indirect Violation — Custom variable name r.Next() missing r.Err().
func P2_Indirect(ctx context.Context, db DB) ([]User, error) {
	r, err := db.Query(ctx, "SELECT id, name FROM users")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var users []User
	for r.Next() { // want `\[ARGUS-A18\] missing r\.Err\(\) check after for r\.Next\(\) loop; unchecked error risks silent dataset truncation on network drop or statement timeout \(CWE-391\)`
		var u User
		_ = r.Scan(&u.ID, &u.Name)
		users = append(users, u)
	}
	return users, nil
}

// P3: Helper Violation — Helper function iterating over cursor without Err().
func P3_Helper(rows Rows) []int {
	var ids []int
	for rows.Next() { // want `\[ARGUS-A18\] missing rows\.Err\(\) check after for rows\.Next\(\) loop; unchecked error risks silent dataset truncation on network drop or statement timeout \(CWE-391\)`
		var id int
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids
}

// P4: Nested Violation — Cursor loop inside branch without Err().
func P4_Nested(ctx context.Context, db DB, filter bool) error {
	if filter {
		rows, _ := db.Query(ctx, "SELECT id FROM users")
		for rows.Next() { // want `\[ARGUS-A18\] missing rows\.Err\(\) check after for rows\.Next\(\) loop; unchecked error risks silent dataset truncation on network drop or statement timeout \(CWE-391\)`
			var id int
			_ = rows.Scan(&id)
		}
	}
	return nil
}

// P5: Alias Violation — Cursor variable cursor.Next() missing cursor.Err().
func P5_Alias(cursor Rows) int {
	count := 0
	for cursor.Next() { // want `\[ARGUS-A18\] missing cursor\.Err\(\) check after for cursor\.Next\(\) loop; unchecked error risks silent dataset truncation on network drop or statement timeout \(CWE-391\)`
		count++
	}
	return count
}

// P_Ignored: Suppressed violation using canonical shortcode.
func P_Ignored(ctx context.Context, db DB) {
	rows, _ := db.Query(ctx, "SELECT 1")
	// argus:ignore-a18 best-effort log streamer
	for rows.Next() {
	}
}
