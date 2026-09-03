package positive

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

// P1: Obvious Violation — plain DSN without timeouts.
func P1_Obvious(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db") // want `\[ARGUS-A12\] pgxpool DSN missing 'statement_timeout' parameter` `\[ARGUS-A12\] pgxpool DSN missing 'lock_timeout' parameter` `\[ARGUS-A12\] pgxpool DSN missing 'idle_in_transaction_session_timeout' parameter`
}

// P2: Indirect Violation — DSN with zero timeout.
func P2_Indirect(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db?statement_timeout=0&lock_timeout=3s&idle_in_transaction_session_timeout=15s") // want `\[ARGUS-A12\] pgxpool DSN parameter 'statement_timeout' must not be set to 0`
}

// P3: Helper Violation — incomplete Config struct literal missing fields.
func P3_Helper() {
	_ = Config{ // want `\[ARGUS-A12\] pgxpool.Config missing ConnConfig.RuntimeParams\["statement_timeout"\]` `\[ARGUS-A12\] pgxpool.Config missing ConnConfig.RuntimeParams\["lock_timeout"\]` `\[ARGUS-A12\] pgxpool.Config missing MaxConnIdleTime` `\[ARGUS-A12\] pgxpool.Config missing MaxConnLifetime`
		ConnConfig: ConnConfig{},
	}
}

// P4: Nested Violation — dynamic assignment with zero statement_timeout.
func P4_Nested(ctx context.Context) {
	cfgZero, _ := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/db")
	cfgZero.MaxConnIdleTime = 5 * time.Minute
	cfgZero.MaxConnLifetime = 1 * time.Hour
	cfgZero.ConnConfig.RuntimeParams["statement_timeout"] = "0"
	cfgZero.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	_, _ = pgxpool.NewWithConfig(ctx, cfgZero) // want `\[ARGUS-A12\] pgxpool.Config timeout parameter 'statement_timeout' must not be set to 0`
}

// P5: Alias Violation — DSN missing lock_timeout parameter.
func P5_Alias(ctx context.Context) {
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db?statement_timeout=10s&idle_in_transaction_session_timeout=15s") // want `\[ARGUS-A12\] pgxpool DSN missing 'lock_timeout' parameter`
}

// P_Ignored: Suppressed violation using verified argus:ignore directive.
func P_Ignored(ctx context.Context) {
	// argus:ignore-a12 offline analytical export worker
	_, _ = pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db")
}
