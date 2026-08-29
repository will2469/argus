package a08

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

func SafeOutboxPattern(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		return tx.Exec(ctx, "INSERT INTO outbox_events (id) VALUES ($1)", "1")
	})
}

func SafeExplicitTx(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := tx.Exec(ctx, "UPDATE balances SET amount = amount + 100 WHERE id = $1", "1"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func BadHttpInBeginFunc(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		_ = tx.Exec(ctx, "UPDATE orders SET status = 'PAID'")
		_, _ = http.Post("https://api.gateway.com/charge", "application/json", nil) // want `\[ARGUS-A08\] blocking external I/O \(http\.Post\) detected inside database transaction`
		return nil
	})
}

func BadSleepInExplicitTx(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_ = tx.Exec(ctx, "UPDATE orders SET status = 'PROCESSING'")
	time.Sleep(2 * time.Second) // want `\[ARGUS-A08\] blocking external I/O \(time\.Sleep\) detected inside database transaction`

	return tx.Commit(ctx)
}

func BadDiskIOInTx(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		_ = tx.Exec(ctx, "INSERT INTO logs (data) VALUES ($1)", "done")
		_ = os.WriteFile("/tmp/out.txt", []byte("data"), 0600) // want `\[ARGUS-A08\] blocking external I/O \(os\.WriteFile\) detected inside database transaction`
		return nil
	})
}

func BadExecInTx(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		_ = exec.Command("echo", "done") // want `\[ARGUS-A08\] blocking external I/O \(exec\.Command\) detected inside database transaction`
		return nil
	})
}

func sendExternalNotification() {
	_, _ = http.Get("https://webhook.site/notify")
}

func BadHelperCallInTx(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		_ = tx.Exec(ctx, "UPDATE users SET verified = true")
		sendExternalNotification() // want `\[ARGUS-A08\] blocking external I/O \(http\.Get\) detected inside database transaction`
		return nil
	})
}

func IgnoredTxIO(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		// argus:ignore ARGUS-A08 mock lab latency injection
		time.Sleep(100 * time.Millisecond)
		return nil
	})
}
