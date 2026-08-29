package a23

import (
	"context"
)

type Config struct {
	ConnConfig ConnConfig
}

type ConnConfig struct {
	RuntimeParams map[string]string
}

type Pool struct{}

func ParseConfig(connString string) (*Config, error) {
	return &Config{}, nil
}

func New(ctx context.Context, dsn string) (*Pool, error) {
	return &Pool{}, nil
}

func NewWithConfig(ctx context.Context, cfg *Config) (*Pool, error) {
	return &Pool{}, nil
}

func setRuntimeParamDefault(cfg *Config, key string, val any) {}

// 1. Safe RuntimeParams map containing transaction_timeout (Compliant)
func SafeRuntimeParams(ctx context.Context) (*Pool, error) {
	cfg := &Config{
		ConnConfig: ConnConfig{
			RuntimeParams: map[string]string{
				"statement_timeout":   "10000",
				"lock_timeout":        "3000",
				"transaction_timeout": "30000",
			},
		},
	}
	return NewWithConfig(ctx, cfg)
}

// 2. Safe DSN with transaction_timeout parameter (Compliant)
func SafeDSN(ctx context.Context) (*Pool, error) {
	const dsn = "postgres://user:pass@localhost:5432/db?sslmode=disable&transaction_timeout=30000"
	return New(ctx, dsn)
}

// 3. Safe Helper call setting transaction_timeout (Compliant)
func SafeHelperSet(ctx context.Context) (*Pool, error) {
	cfg, _ := ParseConfig("postgres://localhost/db")
	setRuntimeParamDefault(cfg, "transaction_timeout", 30000)
	return NewWithConfig(ctx, cfg)
}

// 4. Unsafe RuntimeParams map missing transaction_timeout (Violation)
func UnsafeMissingTxTimeoutMap() map[string]string {
	return map[string]string{ // want `\[ARGUS-A23\] pgxpool RuntimeParams missing 'transaction_timeout' GUC for PostgreSQL 17/18\+ targets; recommend 30000ms-60000ms \(CWE-400\)`
		"statement_timeout": "10000",
		"lock_timeout":      "3000",
	}
}

// 5. Unsafe DSN missing transaction_timeout (Violation)
func UnsafeMissingTxTimeoutDSN(ctx context.Context) (*Pool, error) {
	const dsn = "postgres://user:pass@localhost:5432/db?statement_timeout=10000"
	return New(ctx, dsn) // want `\[ARGUS-A23\] pgxpool DSN missing 'transaction_timeout' parameter for PostgreSQL 17/18\+ targets; recommend 30000ms-60000ms \(CWE-400\)`
}

// 6. Unsafe zero transaction_timeout (Violation)
func UnsafeZeroTxTimeout() map[string]string {
	return map[string]string{ // want `\[ARGUS-A23\] 'transaction_timeout' set to 0; unbounded transaction duration risks XID horizon freezing \(CWE-400\)`
		"statement_timeout":   "10000",
		"transaction_timeout": "0",
	}
}

// 7. Ignored via directive
func IgnoredTxTimeout(ctx context.Context) (*Pool, error) {
	const dsn = "postgres://user:pass@localhost:5432/db?statement_timeout=10000"
	// argus:ignore ARGUS-A23 legacy cluster compatibility
	return New(ctx, dsn)
}

// 8. Ignored via shortcode
func IgnoredShortcode(ctx context.Context) (*Pool, error) {
	const dsn = "postgres://user:pass@localhost:5432/db?statement_timeout=10000"
	// argus:ignore-a23 administrative batch runner
	return New(ctx, dsn)
}
