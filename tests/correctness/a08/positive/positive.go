package positive

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type StorageUploader struct{}

func (s *StorageUploader) Upload(ctx context.Context, key string, data []byte) error {
	return nil
}

type S3Client struct{}

func (s *S3Client) PutObject(ctx context.Context, key string, data []byte) error {
	return nil
}

// P1: Obvious Violation — direct http.Post inside explicit transaction block.
func P1_Obvious(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("UPDATE orders SET status = 'PAID'")
	_, _ = http.Post("https://api.gateway.com/charge", "application/json", nil) // want `\[ARGUS-A08\] blocking external I/O \(http\.Post\) detected inside database transaction`

	return tx.Commit()
}

// P2: Indirect Violation — time.Sleep inside explicit transaction block (database/sql).
func P2_Indirect(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("UPDATE orders SET status = 'PROCESSING'")
	time.Sleep(2 * time.Second) // want `\[ARGUS-A08\] blocking external I/O \(time\.Sleep\) detected inside database transaction`

	return tx.Commit()
}

// P3: Helper Violation — indirect blocking HTTP call via helper function inside transaction.
func P3_Helper(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("UPDATE users SET verified = true")
	sendExternalNotification() // want `\[ARGUS-A08\] blocking external I/O \(http\.Get\) detected inside database transaction`

	return tx.Commit()
}

func sendExternalNotification() {
	_, _ = http.Get("https://webhook.site/notify")
}

// P4: Nested Violation — blocking disk I/O (os.WriteFile) inside transaction block.
func P4_Nested(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = tx.Exec("INSERT INTO logs (data) VALUES ($1)", "done")
	_ = os.WriteFile("/tmp/out.txt", []byte("data"), 0600) // want `\[ARGUS-A08\] blocking external I/O \(os\.WriteFile\) detected inside database transaction`

	return tx.Commit()
}

// P5: Alias Violation — blocking command execution (exec.Command) inside transaction.
func P5_Alias(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_ = exec.Command("echo", "done") // want `\[ARGUS-A08\] blocking external I/O \(exec\.Command\) detected inside database transaction`

	return tx.Commit()
}

// P6: Storage PutObject Violation — cloud storage operation inside transaction block.
func P6_StoragePutObject(ctx context.Context, db *sql.DB, client *S3Client) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_ = client.PutObject(ctx, "invoice.pdf", []byte("data")) // want `\[ARGUS-A08\] blocking external I/O \(storage\.PutObject\) detected inside database transaction`

	return tx.Commit()
}

// P7: Net Dial Violation — blocking network socket dial inside explicit transaction block.
func P7_NetDial(ctx context.Context, db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	conn, _ := net.Dial("tcp", "10.0.0.1:8080") // want `\[ARGUS-A08\] blocking external I/O \(net\.Dial\) detected inside database transaction`
	if conn != nil {
		_ = conn.Close()
	}

	return tx.Commit()
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// argus:ignore ARGUS-A08 mock lab latency injection
	time.Sleep(100 * time.Millisecond)

	return tx.Commit()
}
