package positive

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

type User struct {
	ID    string
	Name  string
	Email string
}

// P1: Obvious Violation — Unbounded dynamic multi-row VALUES batch construction without chunking.
func P1_Obvious(ctx context.Context, db DB, users []User) error {
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

// P2: Indirect Violation — Unbounded dynamic IN clause placeholder generation.
func P2_Indirect(ctx context.Context, db DB, ids []string) error {
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

// P3: Helper Violation — Helper building dynamic VALUES without chunking.
func P3_Helper(ctx context.Context, db DB, users []User) error {
	var vals []string
	var params []any
	for idx, usr := range users {
		vals = append(vals, fmt.Sprintf("($%d, $%d)", idx*2+1, idx*2+2))
		params = append(params, usr.ID, usr.Name)
	}
	stmt := "INSERT INTO members (id, name) VALUES " + strings.Join(vals, ", ")
	_, err := db.Exec(ctx, stmt, params...) // want `\[ARGUS-A20\] unbounded dynamic multi-row VALUES batch construction without chunking; risk of exceeding 65,535 bind parameter limit; recommend pgx.CopyFrom \(CWE-400\)`
	return err
}

// P4: Nested Violation — Inside conditional branch building dynamic IN clause.
func P4_Nested(ctx context.Context, db DB, ids []string, activeOnly bool) error {
	if activeOnly {
		var ph []string
		var args []any
		for i, id := range ids {
			ph = append(ph, fmt.Sprintf("$%d", i+1))
			args = append(args, id)
		}
		q := "SELECT id FROM orders WHERE status = 'active' AND id IN (" + strings.Join(ph, ",") + ")"
		_, err := db.Query(ctx, q, args...) // want `\[ARGUS-A20\] unbounded dynamic IN clause placeholder generation; risk of exceeding 65,535 bind parameter limit; recommend 'WHERE col = ANY\(\$1\)' \(CWE-400\)`
		return err
	}
	return nil
}

// P5: Alias Violation — Multi-row VALUES built into local query variable.
func P5_Alias(ctx context.Context, db DB, users []User) error {
	var ph []string
	var args []any
	for i, u := range users {
		ph = append(ph, fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		args = append(args, u.ID, u.Name)
	}
	sqlStr := "INSERT INTO accounts (id, name) VALUES " + strings.Join(ph, ",")
	finalQuery := sqlStr
	_, err := db.Exec(ctx, finalQuery, args...) // want `\[ARGUS-A20\] unbounded dynamic multi-row VALUES batch construction without chunking; risk of exceeding 65,535 bind parameter limit; recommend pgx.CopyFrom \(CWE-400\)`
	return err
}

// P_Ignored: Suppressed violation using canonical shortcode.
func P_Ignored(ctx context.Context, db DB, users []User) error {
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
