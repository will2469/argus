package adversarial

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type Pool interface {
	Begin(ctx context.Context) (*sql.Tx, error)
	BeginFunc(ctx context.Context, fn func(*sql.Tx) error) error
}

// A1: Branch — conditional blocking sleep inside transaction branch.
func A1_Branch(ctx context.Context, pool Pool, shouldSleep bool) error {
	return pool.BeginFunc(ctx, func(tx *sql.Tx) error {
		if shouldSleep {
			time.Sleep(500 * time.Millisecond)
		}
		_, err := tx.Exec("UPDATE queue SET status = 'DONE'")
		return err
	})
}

// A2: Reassignment / Explicit Tx — blocking file read inside explicit transaction.
func A2_ExplicitTxIO(ctx context.Context, pool Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, _ = os.ReadFile("/etc/hosts")
	return tx.Commit()
}

// A3: Alias & Call Graph — nested local helper invoking blocking HTTP.
func A3_CallGraph(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx *sql.Tx) error {
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
	return m.pool.BeginFunc(ctx, func(tx *sql.Tx) error {
		_, _ = http.Post("https://stripe.com/charge", "application/json", nil)
		return nil
	})
}

// A5: Nested Function — closure inside transaction invoking blocking I/O.
func A5_NestedFunction(ctx context.Context, pool Pool) error {
	return pool.BeginFunc(ctx, func(tx *sql.Tx) error {
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
	return r.pool.BeginFunc(ctx, func(tx *sql.Tx) error {
		_ = exec.Command("ls")
		return nil
	})
}

type StorageUploader struct{}

func (s *StorageUploader) Upload(ctx context.Context, name string, content []byte) error {
	return nil
}

// A7: Storage Upload — cloud storage upload inside transaction closure.
func A7_StorageUpload(ctx context.Context, pool Pool, storage *StorageUploader) error {
	return pool.BeginFunc(ctx, func(tx *sql.Tx) error {
		_ = storage.Upload(ctx, "report.csv", []byte("a,b,c"))
		return nil
	})
}

type Calculator struct{}

func (c *Calculator) Upload(val int) {}

// A8: Calculator Upload — in-memory compute method must NEVER be treated as storage I/O.
func A8_CalculatorUploadInTx(ctx context.Context, pool Pool, calc *Calculator) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	calc.Upload(42)
	return tx.Commit()
}

type S3Client interface {
	PutObject(ctx context.Context, key string, data []byte) error
}

// A9: Client PutObject — PutObject on client must be flagged as external storage I/O.
func A9_ClientPutObjectInTx(ctx context.Context, pool Pool, client S3Client) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_ = client.PutObject(ctx, "report.pdf", []byte("data"))
	return tx.Commit()
}

// A10: Fake Tx Spoofing — custom non-DB interface resembling transaction must NOT trigger violation.
type FakeTx interface {
	Exec(string) error
	Commit() error
	Rollback() error
}

type FakePool interface {
	Begin(ctx context.Context) (FakeTx, error)
	BeginFunc(ctx context.Context, fn func(FakeTx) error) error
}

func A10_FakeTxSpoofing_MustBeSafe(ctx context.Context, fp FakePool) error {
	return fp.BeginFunc(ctx, func(tx FakeTx) error {
		time.Sleep(100 * time.Millisecond) // Safe: FakeTx is NOT a database transaction!
		return nil
	})
}

