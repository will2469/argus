package negative

import (
	"context"
	"net/http"
	"sync"
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

// N6: Channel Synchronization — channel send and receive inside transaction (not external I/O).
func N6_ChannelSync(ctx context.Context, pool Pool, ch chan int) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		ch <- 42
		<-ch
		return tx.Exec(ctx, "UPDATE queue SET processed = true")
	})
}

// N7: Mutex Synchronization — in-memory mutex locking inside transaction (not external I/O).
func N7_MutexLock(ctx context.Context, pool Pool, mu *sync.Mutex) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		mu.Lock()
		defer mu.Unlock()
		return tx.Exec(ctx, "UPDATE balances SET locked = true")
	})
}

// Calculator is an in-memory utility without external storage.
type Calculator struct{}

func (c *Calculator) Upload(val int) {}

// N8: Non-Storage Method Call — calculator.Upload is an in-memory compute method, NOT cloud storage I/O.
func N8_NonStorageUpload(ctx context.Context, pool Pool, calc *Calculator) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		calc.Upload(999)
		return tx.Exec(ctx, "UPDATE metrics SET value = 999")
	})
}

// Parser is a non-database object that defines Begin/Commit.
type Parser struct{}

func (p *Parser) Begin() (*Parser, error) { return p, nil }
func (p *Parser) Commit() error           { return nil }

// N9: Non-Database Object — parser.Begin is not a database transaction.
func N9_NonDBTransaction(parser *Parser) error {
	p, _ := parser.Begin()
	time.Sleep(10 * time.Millisecond) // Safe: parser is NOT a database transaction
	return p.Commit()
}

// WorkflowRunner is a non-database interface with a Begin method.
type WorkflowRunner interface {
	Begin(ctx context.Context) (WorkflowRun, error)
}

type WorkflowRun interface {
	Status() string
}

// N10: Custom Non-DB Interface with Begin() — must not be classified as a DB pool.
func N10_NonDBInterfaceWithBegin(ctx context.Context, runner WorkflowRunner) error {
	run, _ := runner.Begin(ctx)
	time.Sleep(10 * time.Millisecond) // Safe: runner is NOT a database transaction
	_ = run
	return nil
}

// VideoPlayer is a non-database interface with a parameterless Begin method.
type VideoPlayer interface {
	Begin()
}

// N11: VideoPlayer interface with Begin() — must not be classified as a DB pool.
func N11_VideoPlayerInterface(player VideoPlayer) {
	player.Begin()
	_, _ = http.Get("https://example.com/stream") // Safe: player is NOT a DB pool
}
