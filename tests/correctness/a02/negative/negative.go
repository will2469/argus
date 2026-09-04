package negative

import (
	"context"
)

// Rows represents an active database cursor requiring explicit close.
type Rows interface {
	Close()
	Next() bool
}

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
}

type CollectHelper struct{}

func (CollectHelper) CollectRows(rows Rows) error {
	rows.Close()
	return nil
}

var pgx CollectHelper

// N1: Obvious Safe — standard deferred rows.Close().
func N1_ObviousSafe(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	defer rows.Close()
	return nil
}

// N2: Legitimate Idiom — auto-closing helper consuming the rows cursor.
func N2_LegitimateIdiom(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	return pgx.CollectRows(rows)
}

// N3: Unrelated API — non-database structs with Query() method.
type SearchEngine struct{}

func (SearchEngine) Query(q string) (string, error) {
	return q, nil
}

func N3_UnrelatedAPI(search SearchEngine, q string) error {
	res, err := search.Query(q)
	if err != nil {
		return err
	}
	_ = res
	return nil
}

// N4: Sanitized/Protected — deferred close inside an anonymous function closure.
func N4_ClosureDefer(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM users")
	if err != nil {
		return err
	}
	defer func() {
		rows.Close()
	}()
	return nil
}

// N5: Static/Non-Row Operation — database Exec call producing no cursor.
type Execer interface {
	Exec(ctx context.Context, sql string) error
}

func N5_ExecCall(ctx context.Context, db Execer) error {
	return db.Exec(ctx, "DELETE FROM sessions WHERE expired = true")
}

// N6: Clean Alias — cursor assigned to alias and closed unconditionally via defer.
func N6_CleanAlias(ctx context.Context, db DBExecutor) error {
	rows, err := db.Query(ctx, "SELECT id FROM audit_records")
	if err != nil {
		return err
	}
	cursor := rows
	defer cursor.Close()
	return nil
}

// N7: Non-database Query API — receiver returning non-database result without Close method.
type ElasticsearchClient struct{}

func (ElasticsearchClient) Query(ctx context.Context, q string) (string, error) {
	return q, nil
}

func N7_SearchQueryAPI(ctx context.Context, es ElasticsearchClient) error {
	res, err := es.Query(ctx, "filter")
	if err != nil {
		return err
	}
	_ = res
	return nil
}
