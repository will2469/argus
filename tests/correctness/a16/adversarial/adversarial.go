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

// A8: Execution Order — MaxConns assigned after NewWithConfig.
func A8_ExecutionOrder(ctx context.Context) {
	cfgPost, _ := pgxpool.ParseConfig("postgres://localhost/db")
	_, _ = pgxpool.NewWithConfig(ctx, cfgPost)
	cfgPost.MaxConns = 20
}

// A9: Branch Imbalance — MaxConns configured only in one branch without else.
func A9_BranchImbalance(ctx context.Context, isProd bool) {
	cfgBranch, _ := pgxpool.ParseConfig("postgres://localhost/db")
	if isProd {
		cfgBranch.MaxConns = 20
	}
	_, _ = pgxpool.NewWithConfig(ctx, cfgBranch)
}

func ValidatePoolConfig(cfg *Config) {
	// Passive validator: does not set MaxConns
}

// A10: Non-Mutating Helper — helper does not set MaxConns.
func A10_NonMutatingHelper(ctx context.Context) {
	cfgHelper, _ := pgxpool.ParseConfig("postgres://localhost/db")
	ValidatePoolConfig(cfgHelper)
	_, _ = pgxpool.NewWithConfig(ctx, cfgHelper)
}

func getRuntimeConns() int32 {
	return -999
}

// A11: Unverified Dynamic — dynamic unprovable function assigned to MaxConns.
func A11_UnverifiedDynamic(ctx context.Context) {
	cfgDyn, _ := pgxpool.ParseConfig("postgres://localhost/db")
	cfgDyn.MaxConns = getRuntimeConns()
	_, _ = pgxpool.NewWithConfig(ctx, cfgDyn)
}

// A12: Multi-Hop Alias — transitive alias chain without pool_max_conns.
func A12_MultiHopAlias(ctx context.Context) {
	rawDSN := "postgres://multihop:5432/db"
	alias1 := rawDSN
	alias2 := alias1
	_, _ = pgxpool.New(ctx, alias2)
}

func ApplyConfigPartial(cfg *Config, bad bool) {
	if bad {
		cfg.MaxConns = 500
	}
}

// A13: Helper Partial Branch — helper only configures in branch and sets invalid limit.
func A13_HelperPartialBranch(ctx context.Context, bad bool) {
	cfg, _ := pgxpool.ParseConfig("postgres://localhost/db")
	ApplyConfigPartial(cfg, bad)
	_, _ = pgxpool.NewWithConfig(ctx, cfg)
}

func ApplyConfigUnsafeBranch(cfg *Config, bad bool) {
	cfg.MaxConns = 20
	if bad {
		cfg.MaxConns = 500
	}
}

// A14: Helper Unsafe Branch — helper has branch with excessive connection limit.
func A14_HelperUnsafeBranch(ctx context.Context, bad bool) {
	cfg, _ := pgxpool.ParseConfig("postgres://localhost/db")
	ApplyConfigUnsafeBranch(cfg, bad)
	_, _ = pgxpool.NewWithConfig(ctx, cfg)
}


