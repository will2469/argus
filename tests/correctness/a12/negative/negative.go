package negative

import (
	"context"
	"time"
)

type ConfigInner struct {
	RuntimeParams map[string]string
}

type ConnConfig struct {
	RuntimeParams map[string]string
	Config        ConfigInner
}

type Config struct {
	ConnConfig      ConnConfig
	MaxConnIdleTime time.Duration
	MaxConnLifetime time.Duration
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
	return &Config{
		ConnConfig: ConnConfig{
			RuntimeParams: make(map[string]string),
		},
	}, nil
}

// N1: Obvious Safe — DSN with all required timeouts configured.
func N1_ObviousSafe(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db?statement_timeout=10s&lock_timeout=3s&idle_in_transaction_session_timeout=15s")
}

// N2: Legitimate Idiom — fully configured dynamic Config with non-zero timeouts.
func N2_LegitimateIdiom(ctx context.Context) {
	cfg, _ := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/db")
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	_, _ = pgxpool.NewWithConfig(ctx, cfg)
}

// N3: Unrelated API — custom connection pool or non-pgxpool client.
type CustomClient struct{}

func (CustomClient) New(ctx context.Context, dsn string) error {
	return nil
}

func N3_UnrelatedAPI(ctx context.Context, client CustomClient) {
	_ = client.New(ctx, "postgres://user:pass@localhost:5432/db")
}

// N4: Key-Value DSN — key=value formatted DSN with timeouts.
func N4_KeyValueDSN(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "host=localhost user=postgres statement_timeout=5000 lock_timeout=2000 idle_in_transaction_session_timeout=10000")
}

// N5: DefaultConfig Builder — allowed constructor initializing config skeleton.
func DefaultConfig() *Config {
	return &Config{
		ConnConfig: ConnConfig{
			RuntimeParams: make(map[string]string),
		},
	}
}
