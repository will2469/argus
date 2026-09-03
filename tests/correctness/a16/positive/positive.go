package positive

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

// P1: Obvious Violation — DSN without pool_max_conns.
func P1_Obvious(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db") // want `\[ARGUS-A16\] Missing explicit pool_max_conns in DSN`
}

// P2: Indirect Violation — DSN with dangerous pool_max_conns exceeding threshold.
func P2_Indirect(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db?pool_max_conns=500") // want `\[ARGUS-A16\] pool_max_conns \(500\) in DSN exceeds safe direct connection threshold`
}

// P3: Helper Violation — Config missing explicit MaxConns.
func P3_Helper(ctx context.Context) {
	cfgMissing, _ := pgxpool.ParseConfig("postgres://localhost/db")
	_, _ = pgxpool.NewWithConfig(ctx, cfgMissing) // want `\[ARGUS-A16\] pgxpool\.Config missing explicit MaxConns`
}

// P4: Nested Violation — dynamic assignment exceeding threshold.
func P4_Nested(ctx context.Context) {
	cfgGiant, _ := pgxpool.ParseConfig("postgres://localhost/db")
	cfgGiant.MaxConns = 500
	_, _ = pgxpool.NewWithConfig(ctx, cfgGiant) // want `\[ARGUS-A16\] MaxConns \(500\) exceeds safe direct connection limit`
}

// P5: Alias Violation — dynamic assignment set to zero.
func P5_Alias(ctx context.Context) {
	cfgZero, _ := pgxpool.ParseConfig("postgres://localhost/db")
	cfgZero.MaxConns = 0
	_, _ = pgxpool.NewWithConfig(ctx, cfgZero) // want `\[ARGUS-A16\] MaxConns cannot be zero or negative`
}

// P_Ignored: Suppressed violation via canonical shortcode directive.
func P_Ignored(ctx context.Context) {
	cfgIgnored, _ := pgxpool.ParseConfig("postgres://localhost/db")
	// argus:ignore-a16 routed via pgbouncer transaction pooler
	cfgIgnored.MaxConns = 500
	_, _ = pgxpool.NewWithConfig(ctx, cfgIgnored)
}
