package adversarial

import (
	"context"
)

type Pool struct{}

func New(ctx context.Context, dsn string) (*Pool, error) {
	return &Pool{}, nil
}

// A1: Branch — conditional DSN missing transaction_timeout.
func A1_Branch(ctx context.Context, prod bool) (*Pool, error) {
	if prod {
		return New(ctx, "postgres://prod/db?statement_timeout=5000")
	}
	return nil, nil
}

// A2: Reassignment — DSN constructed in steps without transaction_timeout.
func A2_Reassignment(ctx context.Context) (*Pool, error) {
	base := "postgres://user:pass@db:5432/app"
	dsn := base + "?statement_timeout=8000"
	return New(ctx, dsn)
}

// A3: Alias — RuntimeParams map assigned to alias.
func A3_Alias() map[string]string {
	params := map[string]string{
		"statement_timeout": "10000",
		"lock_timeout":      "2000",
	}
	alias := params
	return alias
}

// A4: Wrapper — Pool wrapper returning unconfigured connection.
type PoolFactory struct{}

func (f PoolFactory) Create(ctx context.Context) (*Pool, error) {
	return New(ctx, "postgres://localhost/app?statement_timeout=3000")
}

// A5: Nested Function — closure initializing pool without transaction_timeout.
func A5_NestedFunction(ctx context.Context) (*Pool, error) {
	initPool := func() (*Pool, error) {
		return New(ctx, "postgres://cluster/db?statement_timeout=4000")
	}
	return initPool()
}

// A6: Generic — generic connection manager without transaction_timeout.
type ConnMgr[T any] struct{}

func (c ConnMgr[T]) Connect(ctx context.Context) (*Pool, error) {
	return New(ctx, "postgres://generic/db?statement_timeout=6000")
}

// A7: Explicit Zero — DSN explicitly disabling transaction_timeout.
func A7_ExplicitZero(ctx context.Context) (*Pool, error) {
	return New(ctx, "postgres://localhost/db?statement_timeout=10000&transaction_timeout=0")
}
