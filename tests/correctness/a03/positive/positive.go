package positive

import (
	"context"
)

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	BeginTx(ctx context.Context, opts any) (any, error)
}

// P1: Obvious Violation — direct raw context.Background() passed to Query.
func P1_Obvious(db DBExecutor) error {
	_, err := db.Query(context.Background(), "SELECT id FROM users") // want `\[ARGUS-A03\] database operation Query executed with unbounded context`
	return err
}

// P2: Indirect Violation — context.TODO() assigned to local variable before Exec.
func P2_Indirect(db DBExecutor) error {
	ctx := context.TODO()
	_, err := db.Exec(ctx, "DELETE FROM sessions WHERE expired = true") // want `\[ARGUS-A03\] database operation Exec executed with unbounded context`
	return err
}

// P3: Helper Violation — unbounded context inside internal helper subroutine.
func P3_Helper(db DBExecutor) error {
	return executeHelper(db)
}

func executeHelper(db DBExecutor) error {
	_, err := db.BeginTx(context.Background(), nil) // want `\[ARGUS-A03\] database operation BeginTx executed with unbounded context`
	return err
}

// P4: Nested Violation — unbounded context inside loop and conditional block.
func P4_Nested(db DBExecutor, active bool) error {
	if active {
		for i := 0; i < 1; i++ {
			_, err := db.Query(context.TODO(), "SELECT id FROM accounts") // want `\[ARGUS-A03\] database operation Query executed with unbounded context`
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// P5: Alias Violation — context.Background() aliased across multiple variable assignments.
func P5_Alias(db DBExecutor) error {
	raw := context.Background()
	aliasCtx := raw
	_, err := db.Query(aliasCtx, "SELECT count(*) FROM members") // want `\[ARGUS-A03\] database operation Query executed with unbounded context`
	return err
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(db DBExecutor) error {
	// argus:ignore ARGUS-A03 background cleanup daemon with separate cancellation loop
	_, err := db.Query(context.Background(), "SELECT id FROM temp_records")
	return err
}
