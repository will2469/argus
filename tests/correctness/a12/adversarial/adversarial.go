package adversarial

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

// A1: Branch — conditional DSN initialization missing timeouts inside branch.
func A1_Branch(ctx context.Context, useProd bool) {
	if useProd {
		_, _ = pgxpool.New(ctx, "postgres://prod:5432/db")
	}
}

// A2: Reassignment — DSN variable reassigned to insecure string.
func A2_Reassignment(ctx context.Context) {
	dsn := "postgres://user:pass@localhost:5432/db?statement_timeout=10s&lock_timeout=3s&idle_in_transaction_session_timeout=15s"
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

// A4: Wrapper — factory wrapper calling pgxpool.New with unconfigured DSN.
type PoolFactory struct{}

func (PoolFactory) Init(ctx context.Context) error {
	_, err := pgxpool.New(ctx, "postgres://factory:5432/db")
	return err
}

// A5: Nested Function — closure calling pgxpool.New with zero statement timeout.
func A5_NestedFunction(ctx context.Context) {
	initPool := func() (*Pool, error) {
		return pgxpool.New(ctx, "postgres://u:p@localhost:5432/db?statement_timeout=0&lock_timeout=1s&idle_in_transaction_session_timeout=5s")
	}
	_, _ = initPool()
}

// A6: Generic — generic connection initializer calling pgxpool.New with missing timeouts.
type Connector[T any] struct{}

func (Connector[T]) Connect(ctx context.Context) (*Pool, error) {
	return pgxpool.New(ctx, "postgres://gen:5432/db")
}

// A7: Incomplete Struct Literal — bare Config struct literal missing timeouts.
func A7_BareStructLiteral() {
	_ = Config{}
}
