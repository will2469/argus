package negative

import (
	"context"
	"net/http"
	"time"
)

type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Pool interface {
	Begin(ctx context.Context) (Tx, error)
	BeginFunc(ctx context.Context, fn func(Tx) error) error
}

// N1: Obvious Safe — transaction executing purely database DML queries.
func N1_ObviousSafe(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		return tx.Exec(ctx, "INSERT INTO outbox_events (id) VALUES ($1)", "1")
	})
}

// N2: Legitimate Idiom — explicit transaction block with proper commit and rollback.
func N2_LegitimateIdiom(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := tx.Exec(ctx, "UPDATE balances SET amount = amount + 100 WHERE id = $1", "1"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// N3: Unrelated API — external HTTP call executed completely outside transaction scope.
func N3_UnrelatedAPI() error {
	_, err := http.Get("https://api.example.com/health")
	return err
}

// N4: In-Memory Operation — non-blocking in-memory computation inside transaction.
func N4_InMemoryCompute(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		total := 100 * 2
		return tx.Exec(ctx, "UPDATE orders SET total = $1 WHERE id = 1", total)
	})
}

// N5: Post-Commit Outbox Pattern — HTTP call executed strictly after transaction commits.
func N5_PostCommitIO(ctx context.Context, pool Pool) error {
	err := pool.BeginFunc(ctx, func(tx Tx) error {
		return tx.Exec(ctx, "INSERT INTO outbox (event) VALUES ('order.created')")
	})
	if err != nil {
		return err
	}

	// External I/O outside transaction is safe and compliant
	time.Sleep(10 * time.Millisecond)
	_, _ = http.Post("https://webhook.site/dispatch", "application/json", nil)
	return nil
}
