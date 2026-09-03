package negative

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

// N1: Obvious Safe — safe bounded DSN with pool_max_conns.
func N1_ObviousSafe(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db?pool_max_conns=20")
}

// N2: Legitimate Idiom — explicit bounded MaxConns assignment.
func N2_LegitimateIdiom(ctx context.Context) {
	cfgSafe, _ := pgxpool.ParseConfig("postgres://localhost/db")
	cfgSafe.MaxConns = 20
	_, _ = pgxpool.NewWithConfig(ctx, cfgSafe)
}

// N3: Unrelated API — non-pgxpool client.
type CustomClient struct{}

func (CustomClient) New(ctx context.Context, dsn string) error {
	return nil
}

func N3_UnrelatedAPI(ctx context.Context, client CustomClient) {
	_ = client.New(ctx, "postgres://user:pass@localhost:5432/db")
}

// N4: Sanitized Input — key-value formatted DSN with bounded max conns.
func N4_KeyValueDSN(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "host=localhost user=postgres pool_max_conns=25")
}

// N5: Static Constant — dynamic assignment via resolver function.
func N5_ResolvedMaxConns(ctx context.Context) {
	cfgResolved, _ := pgxpool.ParseConfig("postgres://localhost/db")
	cfgResolved.MaxConns = ResolveMaxConns()
	_, _ = pgxpool.NewWithConfig(ctx, cfgResolved)
}
