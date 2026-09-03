package positive

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

// P1: Obvious Violation — RuntimeParams map missing transaction_timeout.
func P1_Obvious() map[string]string {
	return map[string]string{ // want `\[ARGUS-A23\] pgxpool RuntimeParams missing 'transaction_timeout' GUC for PostgreSQL 17/18\+ targets; recommend 30000ms-60000ms \(CWE-400\)`
		"statement_timeout": "10000",
		"lock_timeout":      "3000",
	}
}

// P2: Indirect Violation — DSN missing transaction_timeout.
func P2_Indirect(ctx context.Context) (*Pool, error) {
	const dsn = "postgres://user:pass@localhost:5432/db?statement_timeout=10000"
	return New(ctx, dsn) // want `\[ARGUS-A23\] pgxpool DSN missing 'transaction_timeout' parameter for PostgreSQL 17/18\+ targets; recommend 30000ms-60000ms \(CWE-400\)`
}

// P3: Helper Violation — Zero transaction_timeout in RuntimeParams map.
func P3_Helper() map[string]string {
	return map[string]string{ // want `\[ARGUS-A23\] 'transaction_timeout' set to 0; unbounded transaction duration risks XID horizon freezing \(CWE-400\)`
		"statement_timeout":   "10000",
		"transaction_timeout": "0",
	}
}

// P4: Nested Violation — Zero transaction_timeout in DSN query string.
func P4_Nested(ctx context.Context, active bool) (*Pool, error) {
	if active {
		const dsn = "postgres://localhost/db?statement_timeout=10000&transaction_timeout=0"
		return New(ctx, dsn) // want `\[ARGUS-A23\] DSN 'transaction_timeout' set to 0; unbounded transaction duration risks XID horizon freezing \(CWE-400\)`
	}
	return nil, nil
}

// P5: Alias Violation — RuntimeParams with lock timeouts but missing transaction_timeout.
func P5_Alias() map[string]string {
	return map[string]string{ // want `\[ARGUS-A23\] pgxpool RuntimeParams missing 'transaction_timeout' GUC for PostgreSQL 17/18\+ targets; recommend 30000ms-60000ms \(CWE-400\)`
		"lock_timeout":                        "5000",
		"idle_in_transaction_session_timeout": "15000",
	}
}

// P_Ignored: Suppressed violation using canonical shortcode.
func P_Ignored(ctx context.Context) (*Pool, error) {
	const dsn = "postgres://user:pass@localhost:5432/db?statement_timeout=10000"
	// argus:ignore-a23 administrative batch runner
	return New(ctx, dsn)
}
