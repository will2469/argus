package adversarial

import (
	"context"
	"net/http"
	"os"
	"os/exec"
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

// A1: Branch — conditional blocking sleep inside transaction branch.
func A1_Branch(ctx context.Context, pool Pool, shouldSleep bool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		if shouldSleep {
			time.Sleep(500 * time.Millisecond)
		}
		return tx.Exec(ctx, "UPDATE queue SET status = 'DONE'")
	})
}

// A2: Reassignment / Explicit Tx — blocking file read inside explicit transaction.
func A2_ExplicitTxIO(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, _ = os.ReadFile("/etc/hosts")
	return tx.Commit(ctx)
}

// A3: Alias & Call Graph — nested local helper invoking blocking HTTP.
func A3_CallGraph(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		callDownstreamAPI()
		return nil
	})
}

func callDownstreamAPI() {
	_, _ = http.Get("https://downstream.internal/sync")
}

// A4: Wrapper — transaction manager struct wrapping BeginFunc with blocking HTTP.
type TxManager struct {
	pool Pool
}

func (m *TxManager) ExecuteWithCharge(ctx context.Context) error {
	return m.pool.BeginFunc(ctx, func(tx Tx) error {
		_, _ = http.Post("https://stripe.com/charge", "application/json", nil)
		return nil
	})
}

// A5: Nested Function — closure inside transaction invoking blocking I/O.
func A5_NestedFunction(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		delay := func() {
			time.Sleep(50 * time.Millisecond)
		}
		delay()
		return nil
	})
}

// A6: Generic — generic transaction runner executing external command.
type Runner[T any] struct {
	pool Pool
}

func (r *Runner[T]) Run(ctx context.Context) error {
	return r.pool.BeginFunc(ctx, func(tx Tx) error {
		_ = exec.Command("ls")
		return nil
	})
}

// A7: Channel I/O — channel send inside transaction causing potential lockup.
func A7_ChannelSend(ctx context.Context, pool Pool, ch chan struct{}) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		ch <- struct{}{}
		return nil
	})
}
