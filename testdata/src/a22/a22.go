package a22

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

// 1. Safe Serializable transaction inside retry loop (Compliant)
func SafeRetryLoopSerializable(ctx context.Context, db DB) error {
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

// 2. Safe RepeatableRead inside retry helper (Compliant)
func WithRetrySerializable(ctx context.Context, db DB, fn func(tx Tx) error) error {
	opts := TxOptions{IsoLevel: RepeatableRead}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	return fn(tx)
}

// 3. Safe ReadCommitted transaction without retry (Compliant)
func SafeReadCommitted(ctx context.Context, db DB) error {
	opts := TxOptions{IsoLevel: ReadCommitted}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// 4. Unsafe single-shot Serializable transaction without retry (Violation)
func UnsafeSingleShotSerializable(ctx context.Context, db DB) error {
	opts := TxOptions{IsoLevel: Serializable}
	tx, err := db.BeginTx(ctx, opts) // want `\[ARGUS-A22\] single-shot 'BeginTx' transaction with strict isolation without automated retry loop; risk of unhandled serialization abort \(SQLSTATE 40001, CWE-362\)`
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// 5. Unsafe single-shot RepeatableRead transaction without retry (Violation)
func UnsafeSingleShotRepeatableRead(ctx context.Context, db DB) error {
	tx, err := db.BeginTx(ctx, TxOptions{IsoLevel: RepeatableRead}) // want `\[ARGUS-A22\] single-shot 'BeginTx' transaction with strict isolation without automated retry loop; risk of unhandled serialization abort \(SQLSTATE 40001, CWE-362\)`
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// 6. Ignored Serializable transaction via directive
func IgnoredSerializable(ctx context.Context, db DB) error {
	opts := TxOptions{IsoLevel: Serializable}
	// argus:ignore ARGUS-A22 intentional single-shot test assertion
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// 7. Ignored Serializable transaction via shortcode
func IgnoredShortcode(ctx context.Context, db DB) error {
	// argus:ignore-a22 manual admin execution
	tx, err := db.BeginTx(ctx, TxOptions{IsoLevel: Serializable})
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
