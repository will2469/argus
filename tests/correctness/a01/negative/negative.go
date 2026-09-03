package negative

import (
	"context"
	"strings"
)

// DBExecutor represents a standard database interface for queries.
type DBExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	QueryRow(ctx context.Context, sql string, args ...any) any
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

// N1: Obvious Safe — standard parameterized query with placeholders.
func N1_ObviousSafe(ctx context.Context, db DBExecutor, id string) {
	_, _ = db.Query(ctx, "SELECT id, name FROM users WHERE id = $1", id)
}

// N2: Legitimate Idiom — compile-time string literal concatenation across lines.
func N2_LegitimateIdiom(ctx context.Context, db DBExecutor) {
	_, _ = db.Query(ctx, "SELECT id, name, email "+
		"FROM users "+
		"WHERE active = true AND deleted_at IS NULL")
}

// N3: Unrelated API — non-database structs sharing method names (Query/Exec).
type Logger struct{}

func (Logger) Exec(ctx context.Context, msg string) {}

type SearchEngine struct{}

func (SearchEngine) Query(ctx context.Context, q string) string { return q }

func N3_UnrelatedAPI(ctx context.Context, logger Logger, search SearchEngine, userInput string) {
	logger.Exec(ctx, "User action: "+userInput)
	search.Query(ctx, "prefix:"+userInput)
}

// N4: Sanitized Input — table name verified and sanitized through identifier sanitizer.
func SanitizeIdentifier(ident string) string {
	return `"` + strings.ReplaceAll(ident, `"`, `""`) + `"`
}

func N4_SanitizedInput(ctx context.Context, db DBExecutor, tableName string) {
	_, _ = db.Query(ctx, "SELECT * FROM "+SanitizeIdentifier(tableName))
}

// N5: Static/Constant Input — compile-time constant string query.
const ActiveUsersQuery = "SELECT id, email FROM users WHERE is_active = true"

func N5_StaticConstant(ctx context.Context, db DBExecutor) {
	_, _ = db.Query(ctx, ActiveUsersQuery)
}
