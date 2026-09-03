package negative

import (
	"context"
	"net/http"
	"time"
)

// DBExecutor represents a database query engine interface.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	BeginTx(ctx context.Context, opts any) (any, error)
}

// N1: Obvious Safe — HTTP request scoped context with client cancellation.
func N1_ObviousSafe(r *http.Request, db DBExecutor) error {
	_, err := db.Query(r.Context(), "SELECT id FROM users")
	return err
}

// N2: Legitimate Idiom — bounded timeout context.
func N2_LegitimateIdiom(db DBExecutor) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := db.Exec(ctx, "UPDATE users SET active = true")
	return err
}

// N3: Unrelated API — non-database structs receiving context.Background().
type SearchEngine struct{}

func (SearchEngine) Query(ctx context.Context, term string) (string, error) {
	return term, nil
}

func N3_UnrelatedAPI(search SearchEngine) error {
	_, err := search.Query(context.Background(), "golang")
	return err
}

// N4: Sanitized / Delegated — context passed down from caller parameters.
func N4_CallerContext(ctx context.Context, db DBExecutor) error {
	_, err := db.Query(ctx, "SELECT id FROM accounts WHERE active = true")
	return err
}

// N5: Static Constant / URL Query — URL query params without database interaction.
func N5_URLQuery(r *http.Request) string {
	return r.URL.Query().Get("page")
}
