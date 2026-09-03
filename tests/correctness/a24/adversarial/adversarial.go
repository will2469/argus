package adversarial

import (
	"context"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}

// A1: Branch — conditional query missing tenant_id predicate.
func A1_Branch(ctx context.Context, db DB, cond bool) {
	if cond {
		_, _ = db.Query(ctx, "SELECT id, name FROM users WHERE status = 'PENDING'")
	}
}

// A2: Reassignment — query variable assigned across steps.
func A2_Reassignment(ctx context.Context, db DB) {
	q := "SELECT id, total FROM orders"
	_, _ = db.Query(ctx, q)
}

// A3: Alias — table alias used without tenant_id predicate.
func A3_Alias(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT u.id, u.email FROM users u WHERE u.status = 'ACTIVE'")
}

// A4: Wrapper — DAO struct method missing tenant_id predicate.
type AccountRepo struct {
	db DB
}

func (r AccountRepo) FindActive(ctx context.Context) {
	_, _ = r.db.Query(ctx, "SELECT id, name FROM accounts WHERE balance > 0")
}

// A5: Nested Function — closure executing query missing tenant_id predicate.
func A5_NestedFunction(ctx context.Context, db DB, id string) {
	deleteFn := func() {
		_, _ = db.Exec(ctx, "DELETE FROM orders WHERE id = $1", id)
	}
	deleteFn()
}

// A6: Generic — generic updater missing tenant_id predicate.
type UserManager[T any] struct {
	db DB
}

func (m UserManager[T]) DeactivateAll(ctx context.Context) {
	_, _ = m.db.Exec(ctx, "UPDATE users SET status = 'INACTIVE'")
}

// A7: Null Test Evasion — IS NOT NULL pseudo-predicate attempt.
func A7_NullTestEvasion(ctx context.Context, db DB) {
	_, _ = db.Query(ctx, "SELECT id, email FROM users WHERE tenant_id IS NOT NULL")
}
