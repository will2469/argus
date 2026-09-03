package negative

import (
	"context"
	"errors"
)

type TxIsoLevel string

const (
	Serializable    TxIsoLevel = "Serializable"
	RepeatableRead  TxIsoLevel = "RepeatableRead"
	ReadCommitted   TxIsoLevel = "ReadCommitted"
	ReadUncommitted TxIsoLevel = "ReadUncommitted"
)

type TxOptions struct {
	IsoLevel TxIsoLevel
}

type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type DB struct{}

func (DB) BeginTx(ctx context.Context, opts TxOptions) (Tx, error) {
	return nil, nil
}

func (DB) Query(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

// N1: Obvious Safe — Serializable transaction inside retry loop.
func N1_ObviousSafe(ctx context.Context, db DB) error {
	opts := TxOptions{IsoLevel: Serializable}
	for attempt := 1; attempt <= 3; attempt++ {
		tx, err := db.BeginTx(ctx, opts)
		if err != nil {
			continue
		}
		_ = tx.Commit(ctx)
		return nil
	}
	return errors.New("failed after retries")
}

// N2: Legitimate Idiom — RepeatableRead inside WithRetry helper.
func WithRetrySerializable(ctx context.Context, db DB, fn func(tx Tx) error) error {
	opts := TxOptions{IsoLevel: RepeatableRead}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	return fn(tx)
}

// N3: Permitted Isolation — ReadCommitted transaction does not require retry loop.
func N3_ReadCommitted(ctx context.Context, db DB) error {
	opts := TxOptions{IsoLevel: ReadCommitted}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// N4: Permitted Isolation — ReadUncommitted transaction does not require retry loop.
func N4_ReadUncommitted(ctx context.Context, db DB) error {
	opts := TxOptions{IsoLevel: ReadUncommitted}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// N5: Non-Transactional — Regular database query without BeginTx.
func N5_NonTransactional(ctx context.Context, db DB) error {
	_, err := db.Query(ctx, "SELECT 1")
	return err
}
