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
