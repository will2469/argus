package positive

import (
	"context"
	"net"
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

type StorageUploader struct{}

func (s *StorageUploader) Upload(ctx context.Context, key string, data []byte) error {
	return nil
}

// P1: Obvious Violation — direct http.Post inside BeginFunc transaction closure.
func P1_Obvious(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		_ = tx.Exec(ctx, "UPDATE orders SET status = 'PAID'")
		_, _ = http.Post("https://api.gateway.com/charge", "application/json", nil) // want `\[ARGUS-A08\] blocking external I/O \(http\.Post\) detected inside database transaction`
		return nil
	})
}

// P2: Indirect Violation — time.Sleep inside explicit transaction block (Begin ... Commit).
func P2_Indirect(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_ = tx.Exec(ctx, "UPDATE orders SET status = 'PROCESSING'")
	time.Sleep(2 * time.Second) // want `\[ARGUS-A08\] blocking external I/O \(time\.Sleep\) detected inside database transaction`

	return tx.Commit(ctx)
}

// P3: Helper Violation — indirect blocking HTTP call via helper function inside transaction.
func P3_Helper(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		_ = tx.Exec(ctx, "UPDATE users SET verified = true")
		sendExternalNotification() // want `\[ARGUS-A08\] blocking external I/O \(http\.Get\) detected inside database transaction`
		return nil
	})
}

func sendExternalNotification() {
	_, _ = http.Get("https://webhook.site/notify")
}

// P4: Nested Violation — blocking disk I/O (os.WriteFile) inside transaction closure.
func P4_Nested(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		_ = tx.Exec(ctx, "INSERT INTO logs (data) VALUES ($1)", "done")
		_ = os.WriteFile("/tmp/out.txt", []byte("data"), 0600) // want `\[ARGUS-A08\] blocking external I/O \(os\.WriteFile\) detected inside database transaction`
		return nil
	})
}

// P5: Alias Violation — blocking command execution (exec.Command) inside transaction.
func P5_Alias(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		_ = exec.Command("echo", "done") // want `\[ARGUS-A08\] blocking external I/O \(exec\.Command\) detected inside database transaction`
		return nil
	})
}

// P6: Storage Upload Violation — cloud storage Upload inside transaction closure.
func P6_StorageUpload(ctx context.Context, pool Pool, storage *StorageUploader) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		_ = storage.Upload(ctx, "invoice.pdf", []byte("data")) // want `\[ARGUS-A08\] blocking external I/O \(storage\.Upload\) detected inside database transaction`
		return nil
	})
}

// P7: Net Dial Violation — blocking network socket dial inside explicit transaction block.
func P7_NetDial(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	conn, _ := net.Dial("tcp", "10.0.0.1:8080") // want `\[ARGUS-A08\] blocking external I/O \(net\.Dial\) detected inside database transaction`
	if conn != nil {
		_ = conn.Close()
	}

	return tx.Commit(ctx)
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx Tx) error {
		// argus:ignore ARGUS-A08 mock lab latency injection
		time.Sleep(100 * time.Millisecond)
		return nil
	})
}
