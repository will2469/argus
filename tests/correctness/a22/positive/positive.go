package positive

import (
	"context"
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

// P1: Obvious Violation — Single-shot Serializable transaction without retry loop.
func P1_Obvious(ctx context.Context, db DB) error {
	opts := TxOptions{IsoLevel: Serializable}
	tx, err := db.BeginTx(ctx, opts) // want `\[ARGUS-A22\] single-shot 'BeginTx' transaction with strict isolation without automated retry loop; risk of unhandled serialization abort \(SQLSTATE 40001, CWE-362\)`
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// P2: Indirect Violation — Single-shot RepeatableRead transaction without retry loop.
func P2_Indirect(ctx context.Context, db DB) error {
	tx, err := db.BeginTx(ctx, TxOptions{IsoLevel: RepeatableRead}) // want `\[ARGUS-A22\] single-shot 'BeginTx' transaction with strict isolation without automated retry loop; risk of unhandled serialization abort \(SQLSTATE 40001, CWE-362\)`
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// P3: Helper Violation — Helper starting Serializable tx without enclosing retry.
func P3_Helper(ctx context.Context, db DB) (Tx, error) {
	return db.BeginTx(ctx, TxOptions{IsoLevel: Serializable}) // want `\[ARGUS-A22\] single-shot 'BeginTx' transaction with strict isolation without automated retry loop; risk of unhandled serialization abort \(SQLSTATE 40001, CWE-362\)`
}

// P4: Nested Violation — Inside conditional branch with strict isolation without retry.
func P4_Nested(ctx context.Context, db DB, strict bool) error {
	if strict {
		tx, err := db.BeginTx(ctx, TxOptions{IsoLevel: Serializable}) // want `\[ARGUS-A22\] single-shot 'BeginTx' transaction with strict isolation without automated retry loop; risk of unhandled serialization abort \(SQLSTATE 40001, CWE-362\)`
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	return nil
}

// P5: Alias Violation — Aliased options struct without retry loop.
func P5_Alias(ctx context.Context, db DB) error {
	opt := TxOptions{IsoLevel: Serializable}
	alias := opt
	tx, err := db.BeginTx(ctx, alias) // want `\[ARGUS-A22\] single-shot 'BeginTx' transaction with strict isolation without automated retry loop; risk of unhandled serialization abort \(SQLSTATE 40001, CWE-362\)`
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// P_Ignored: Suppressed violation using canonical shortcode.
func P_Ignored(ctx context.Context, db DB) error {
	// argus:ignore-a22 manual admin execution
	tx, err := db.BeginTx(ctx, TxOptions{IsoLevel: Serializable})
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
