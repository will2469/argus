package positive

import (
	"context"
)

// Rows represents an active database cursor requiring explicit close.
type Rows interface {
	Close()
	Next() bool
}

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

// P1: Obvious Violation — query executed without defer rows.Close().
func P1_Obvious(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users") // want `\[ARGUS-A02\] missing defer rows\.Close\(\) for Query\(\) call`
	if err != nil {
		return err
	}
	_ = rows
	return nil
}

// P2: Indirect Violation — naked rows.Close() without defer (vulnerable to early return / panic).
func P2_Indirect(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users") // want `\[ARGUS-A02\] missing defer rows\.Close\(\) for Query\(\) call`
	if err != nil {
		return err
	}
	rows.Close()
	return nil
}

// P3: Helper Violation — unclosed query inside an internal helper subroutine.
func P3_Helper(ctx context.Context, db DBExecutor) error {
	return executeQueryHelper(ctx, db)
}

func executeQueryHelper(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM profiles") // want `\[ARGUS-A02\] missing defer rows\.Close\(\) for Query\(\) call`
	if err != nil {
		return err
	}
	_ = rows
	return nil
}

// P4: Nested Violation — query inside nested condition and loop without defer.
func P4_Nested(ctx context.Context, db DBExecutor, active bool) error {
	if active {
		for i := 0; i < 1; i++ {
			rows, err := db.Query(ctx, "SELECT id FROM logs") // want `\[ARGUS-A02\] missing defer rows\.Close\(\) for Query\(\) call`
			if err != nil {
				return err
			}
			_ = rows
		}
	}
	return nil
}

// P5: Alias Violation — returning rows variable transfers resource ownership.
func P5_Alias(ctx context.Context, db DBExecutor) (Rows, error) {
	rows, err := db.Query(ctx, "SELECT id FROM accounts") // want `\[ARGUS-A02\] returning rows variable "rows" transfers resource ownership`
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, db DBExecutor) error {
	// argus:ignore ARGUS-A02 intentional mock generator in test fixture
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	_ = rows
	return nil
}
