package negative

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

// N1: Obvious Safe — RuntimeParams map with all timeouts including transaction_timeout.
func N1_ObviousSafe(ctx context.Context) (*Pool, error) {
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

// N2: Legitimate Idiom — Safe DSN with transaction_timeout parameter.
func N2_LegitimateIdiom(ctx context.Context) (*Pool, error) {
	const dsn = "postgres://user:pass@localhost:5432/db?sslmode=disable&transaction_timeout=30000"
	return New(ctx, dsn)
}

// N3: Unrelated API — General application configuration map.
func N3_UnrelatedAPI() map[string]string {
	return map[string]string{
		"app_env":  "production",
		"log_rate": "100",
	}
}

// N4: Helper Set — Safe configuration via helper function.
func N4_HelperSet(ctx context.Context) (*Pool, error) {
	cfg, _ := ParseConfig("postgres://localhost/db")
	setRuntimeParamDefault(cfg, "transaction_timeout", 30000)
	return NewWithConfig(ctx, cfg)
}

// N5: Non-PGX — Standard HTTP client initialization.
func N5_NonPGX() string {
	return "http://localhost:8080/healthz"
}
