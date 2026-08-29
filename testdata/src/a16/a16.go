package a16

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

func ResolveMaxConns() int32 {
	return 20
}

func Cases(ctx context.Context) {
	// 1. DSN without pool_max_conns
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db") // want `\[ARGUS-A16\] Missing explicit pool_max_conns in DSN`

	// 2. DSN with dangerous pool_max_conns (500)
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db?pool_max_conns=500") // want `\[ARGUS-A16\] pool_max_conns \(500\) in DSN exceeds safe direct connection threshold`

	// 3. DSN with zero pool_max_conns
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db?pool_max_conns=0") // want `\[ARGUS-A16\] pool_max_conns in DSN must be a positive integer`

	// 4. Safe DSN
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db?pool_max_conns=20")

	// 5. Config missing MaxConns
	cfgMissing, _ := pgxpool.ParseConfig("postgres://localhost/db")
	_, _ = pgxpool.NewWithConfig(ctx, cfgMissing) // want `\[ARGUS-A16\] pgxpool\.Config missing explicit MaxConns`

	// 6. Dynamic assignment too large
	cfgGiant, _ := pgxpool.ParseConfig("postgres://localhost/db")
	cfgGiant.MaxConns = 500
	_, _ = pgxpool.NewWithConfig(ctx, cfgGiant) // want `\[ARGUS-A16\] MaxConns \(500\) exceeds safe direct connection limit`

	// 7. Dynamic assignment zero
	cfgZero, _ := pgxpool.ParseConfig("postgres://localhost/db")
	cfgZero.MaxConns = 0
	_, _ = pgxpool.NewWithConfig(ctx, cfgZero) // want `\[ARGUS-A16\] MaxConns cannot be zero or negative`

	// 8. Safe dynamic assignment
	cfgSafe, _ := pgxpool.ParseConfig("postgres://localhost/db")
	cfgSafe.MaxConns = 20
	_, _ = pgxpool.NewWithConfig(ctx, cfgSafe)

	// 9. Safe via resolver function
	cfgResolved, _ := pgxpool.ParseConfig("postgres://localhost/db")
	cfgResolved.MaxConns = ResolveMaxConns()
	_, _ = pgxpool.NewWithConfig(ctx, cfgResolved)

	// 10. Ignored via canonical shortcode
	cfgIgnored, _ := pgxpool.ParseConfig("postgres://localhost/db")
	// argus:ignore-a16 routed via pgbouncer transaction pooler
	cfgIgnored.MaxConns = 500
	_, _ = pgxpool.NewWithConfig(ctx, cfgIgnored)
}
