package adversarial

import (
	"context"
)

type Config struct {
	MaxConns int32
}

type Pool struct{}

type pgxpoolPkg struct{}

var pgxpool pgxpoolPkg

func (pgxpoolPkg) New(ctx context.Context, dsn string) (*Pool, error) {
	return &Pool{}, nil
}

func (pgxpoolPkg) NewWithConfig(ctx context.Context, cfg *Config) (*Pool, error) {
	return &Pool{}, nil
}

func (pgxpoolPkg) ParseConfig(dsn string) (*Config, error) {
	return &Config{}, nil
}

// A1: Branch — conditional DSN missing pool_max_conns inside branch.
func A1_Branch(ctx context.Context, useProd bool) {
	if useProd {
		_, _ = pgxpool.New(ctx, "postgres://prod:5432/db")
	}
}

// A2: Reassignment — DSN variable reassigned to unbounded string.
func A2_Reassignment(ctx context.Context) {
	dsn := "postgres://user:pass@localhost:5432/db?pool_max_conns=20"
	_ = dsn
	dsn = "postgres://insecure:5432/db"
	_, _ = pgxpool.New(ctx, dsn)
}

// A3: Alias — DSN string aliased to another variable.
func A3_Alias(ctx context.Context) {
	raw := "postgres://plain:5432/db"
	alias := raw
	_, _ = pgxpool.New(ctx, alias)
}

// A4: Wrapper — factory wrapper calling pgxpool.New without pool_max_conns.
type PoolFactory struct{}

func (PoolFactory) Init(ctx context.Context) error {
	_, err := pgxpool.New(ctx, "postgres://factory:5432/db")
	return err
}

// A5: Nested Function — closure calling pgxpool.New with excessive pool_max_conns.
func A5_NestedFunction(ctx context.Context) {
	initPool := func() (*Pool, error) {
		return pgxpool.New(ctx, "postgres://localhost:5432/db?pool_max_conns=500")
	}
	_, _ = initPool()
}

// A6: Generic — generic connection initializer missing pool_max_conns.
type Connector[T any] struct{}

func (Connector[T]) Connect(ctx context.Context) (*Pool, error) {
	return pgxpool.New(ctx, "postgres://gen:5432/db")
}

// A7: Negative Value — dynamic assignment negative MaxConns.
func A7_NegativeValue(ctx context.Context) {
	cfgNeg, _ := pgxpool.ParseConfig("postgres://localhost/db")
	cfgNeg.MaxConns = -5
	_, _ = pgxpool.NewWithConfig(ctx, cfgNeg)
}
