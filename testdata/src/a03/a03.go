package a03

import (
	"context"
	"net/http"
	"time"
)

type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	BeginTx(ctx context.Context, opts any) (any, error)
}

func SafeHTTPContext(r *http.Request, db DBExecutor) error {
	_, err := db.Query(r.Context(), "SELECT id FROM users")
	return err
}

func SafeTimeoutContext(db DBExecutor) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := db.Exec(ctx, "UPDATE users SET active = true")
	return err
}

func BadDirectBackground(db DBExecutor) error {
	_, err := db.Query(context.Background(), "SELECT id FROM users") // want `\[ARGUS-A03\] database operation Query executed with unbounded context`
	return err
}

func BadDirectTODO(db DBExecutor) error {
	_, err := db.Exec(context.TODO(), "DELETE FROM temp") // want `\[ARGUS-A03\] database operation Exec executed with unbounded context`
	return err
}

func BadIndirectBackground(db DBExecutor) error {
	ctx := context.Background()
	_, err := db.BeginTx(ctx, nil) // want `\[ARGUS-A03\] database operation BeginTx executed with unbounded context`
	return err
}

func BadIndirectTODO(db DBExecutor) error {
	ctx := context.TODO()
	_, err := db.Query(ctx, "SELECT id FROM users") // want `\[ARGUS-A03\] database operation Query executed with unbounded context`
	return err
}

func IgnoredBackground(db DBExecutor) error {
	// argus:ignore ARGUS-A03 background cleanup daemon with separate cancellation loop
	_, err := db.Query(context.Background(), "SELECT id FROM users")
	return err
}
