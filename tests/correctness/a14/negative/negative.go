package negative

import (
	"context"
)

type DB struct{}

func (DB) Query(ctx context.Context, sql string, args ...any) (any, error) {
	return nil, nil
}

func (DB) QueryRow(ctx context.Context, sql string, args ...any) any {
	return nil
}

type Logger struct{}

func (Logger) Info(msg string) {}

type SearchEngine struct{}

func (SearchEngine) Query(ctx context.Context, q string) (any, error) {
	return nil, nil
}

// N1: Obvious Safe — explicit column projection.
func N1_ObviousSafe(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id, nik, nama_lengkap FROM users WHERE is_active = true")
}

// N2: Legitimate Idiom — COUNT(*) aggregate function.
func N2_LegitimateIdiom(ctx context.Context, db DB) {
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM users")
}

// N3: Unrelated API — logger method passing query-like string.
func N3_UnrelatedAPI(logger Logger) {
	logger.Info("SELECT * FROM users")
}

// N4: Sanitized Input — EXISTS subquery with SELECT * (standard SQL idiom).
func N4_ExistsSubquery(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id FROM users WHERE EXISTS (SELECT * FROM profiles WHERE profiles.user_id = users.id)")
}

// N5: Static Constant — NOT EXISTS subquery with SELECT *.
func N5_NotExistsSubquery(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id FROM users WHERE NOT EXISTS (SELECT * FROM profiles WHERE profiles.user_id = users.id)")
}

// N6: Shadowed Safe Query — inner scope shadows outer SELECT * with safe projection.
func N6_ShadowedSafeQuery(ctx context.Context, db DB) {
	query := "SELECT * FROM audit"
	_ = query
	if true {
		query := "SELECT id, name FROM users"
		_, _ = db.Query(ctx, query)
	}
}

var packageBadQuery = "SELECT * FROM audit"

// N7: Package Shadowed Safe — local var shadows package-level SELECT * with safe projection.
func N7_PackageShadowedSafe(ctx context.Context, db DB) {
	_ = packageBadQuery
	packageBadQuery := "SELECT id, name FROM users"
	_, _ = db.Query(ctx, packageBadQuery)
}

// N8: Unrelated Search Engine — non-DB receiver calling Query method with SELECT *.
func N8_UnrelatedSearchEngine(ctx context.Context, engine SearchEngine) {
	_, _ = engine.Query(ctx, "SELECT * FROM index")
}

// N9: Outer Scope Safe — inner scope defines SELECT * on shadowed var, but outer query is safe.
func N9_OuterScopeSafe(ctx context.Context, db DB) {
	query := "SELECT id, name FROM users"
	if true {
		query := "SELECT * FROM audit"
		_ = query
	}
	_, _ = db.Query(ctx, query)
}
