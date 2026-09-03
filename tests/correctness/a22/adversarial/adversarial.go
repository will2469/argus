package adversarial

import (
	"context"
)

type TxIsoLevel string

const (
	Serializable   TxIsoLevel = "Serializable"
	RepeatableRead TxIsoLevel = "RepeatableRead"
)

type TxOptions struct {
	IsoLevel TxIsoLevel
}

type Tx interface {
	Commit(ctx context.Context) error
}

type DB struct{}

func (DB) BeginTx(ctx context.Context, opts TxOptions) (Tx, error) {
	return nil, nil
}

// A1: Branch — conditional selection of Serializable without retry loop.
func A1_Branch(ctx context.Context, db DB, strict bool) {
	if strict {
		_, _ = db.BeginTx(ctx, TxOptions{IsoLevel: Serializable})
	}
}

// A2: Reassignment — options modified to Serializable before call.
func A2_Reassignment(ctx context.Context, db DB) {
	opts := TxOptions{IsoLevel: Serializable}
	finalOpts := opts
	_, _ = db.BeginTx(ctx, finalOpts)
}

// A3: Alias — options struct aliased to new variable.
func A3_Alias(ctx context.Context, db DB) {
	o1 := TxOptions{IsoLevel: RepeatableRead}
	o2 := o1
	_, _ = db.BeginTx(ctx, o2)
}

// A4: Wrapper — struct method starting Serializable transaction.
type TxManager struct {
	db DB
}

func (m TxManager) StartStrict(ctx context.Context) (Tx, error) {
	return m.db.BeginTx(ctx, TxOptions{IsoLevel: Serializable})
}

// A5: Nested Function — closure starting strict transaction.
func A5_NestedFunction(ctx context.Context, db DB) {
	run := func() {
		_, _ = db.BeginTx(ctx, TxOptions{IsoLevel: Serializable})
	}
	run()
}

// A6: Generic — generic transaction runner starting Serializable tx without retry.
type Runner[T any] struct {
	db DB
}

func (r Runner[T]) Exec(ctx context.Context) {
	_, _ = r.db.BeginTx(ctx, TxOptions{IsoLevel: Serializable})
}

// A7: Type Cast — typed literal with explicit conversion.
func A7_TypeCast(ctx context.Context, db DB) {
	_, _ = db.BeginTx(ctx, TxOptions{IsoLevel: TxIsoLevel("Serializable")})
}
