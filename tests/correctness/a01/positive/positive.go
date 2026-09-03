package positive

import (
	"context"
	"fmt"
)

// DBExecutor represents a standard database interface for queries.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	QueryRow(ctx context.Context, sql string, args ...any) any
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

// P1: Obvious Violation — direct inline string concatenation in query call.
func P1_Obvious(ctx context.Context, db DBExecutor, id string) {
	db.Query(ctx, "SELECT * FROM users WHERE id = "+id) // want `\[ARGUS-A01\] unsafe SQL concatenation or formatting`
}

// P2: Indirect Violation — multi-step variable assembly with untrusted taint.
func P2_Indirect(ctx context.Context, db DBExecutor, status string) {
	query := "SELECT * FROM users WHERE status = '"
	query += status + "'"
	db.Query(ctx, query) // want `\[ARGUS-A01\] unsafe SQL concatenation or formatting`
}

// P3: Helper Violation — violation inside an internal helper function.
func P3_Helper(ctx context.Context, db DBExecutor, id string) {
	executeUserDeleteHelper(ctx, db, id)
}

func executeUserDeleteHelper(ctx context.Context, db DBExecutor, id string) {
	sql := fmt.Sprintf("DELETE FROM users WHERE id = '%s'", id)
	db.Exec(ctx, sql) // want `\[ARGUS-A01\] unsafe SQL concatenation or formatting`
}

// P4: Nested Violation — violation deeply nested inside loop and condition.
func P4_Nested(ctx context.Context, db DBExecutor, roles []string) {
	for _, role := range roles {
		if role != "" {
			db.QueryRow(ctx, "SELECT count(*) FROM members WHERE role = '"+role+"'") // want `\[ARGUS-A01\] unsafe SQL concatenation or formatting`
		}
	}
}

// P5: Alias Violation — violation using an aliased DB type.
type AppDB = DBExecutor

func P5_Alias(ctx context.Context, r AppDB, id string) {
	r.Query(ctx, "SELECT * FROM accounts a WHERE a.id = "+id) // want `\[ARGUS-A01\] unsafe SQL concatenation or formatting`
}

// P_Ignored: Violation suppressed with verified argus:ignore directive.
func P_Ignored(ctx context.Context, db DBExecutor, id string) {
	// argus:ignore ARGUS-A01 legacy maintenance script with verified numeric id
	db.Exec(ctx, "DELETE FROM temp_sessions WHERE id = "+id)
}
