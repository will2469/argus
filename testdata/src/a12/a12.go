package a12

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

func TestCases(ctx context.Context) {
	// 1. Plain DSN without timeouts
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db") // want `\[ARGUS-A12\] pgxpool DSN missing 'statement_timeout' parameter` `\[ARGUS-A12\] pgxpool DSN missing 'lock_timeout' parameter` `\[ARGUS-A12\] pgxpool DSN missing 'idle_in_transaction_session_timeout' parameter`

	// 2. DSN with zero timeout
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db?statement_timeout=0&lock_timeout=3s&idle_in_transaction_session_timeout=15s") // want `\[ARGUS-A12\] pgxpool DSN parameter 'statement_timeout' must not be set to 0`

	// 3. Safe DSN with all timeouts (compliant)
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db?statement_timeout=10s&lock_timeout=3s&idle_in_transaction_session_timeout=15s")

	// 4. Incomplete struct literal
	_ = Config{ // want `\[ARGUS-A12\] pgxpool.Config missing ConnConfig.RuntimeParams\["statement_timeout"\]` `\[ARGUS-A12\] pgxpool.Config missing ConnConfig.RuntimeParams\["lock_timeout"\]` `\[ARGUS-A12\] pgxpool.Config missing MaxConnIdleTime` `\[ARGUS-A12\] pgxpool.Config missing MaxConnLifetime`
		ConnConfig: ConnConfig{},
	}

	// 5. Dynamic assignments compliant
	cfg, _ := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/db")
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "10000"
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	_, _ = pgxpool.NewWithConfig(ctx, cfg)

	// 6. Dynamic assignment with zero timeout
	cfgZero, _ := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/db")
	cfgZero.MaxConnIdleTime = 5 * time.Minute
	cfgZero.MaxConnLifetime = 1 * time.Hour
	cfgZero.ConnConfig.RuntimeParams["statement_timeout"] = "0"
	cfgZero.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	_, _ = pgxpool.NewWithConfig(ctx, cfgZero) // want `\[ARGUS-A12\] pgxpool.Config timeout parameter 'statement_timeout' must not be set to 0`

	// 7. Ignored call via canonical shortcode
	// argus:ignore-a12 offline analytical export worker
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db")
}
