package negative

import (
	"context"
)

// DBExecutor represents a standard database interface for queries.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

// N1: Obvious Safe — append-only INSERT into audit log table.
func N1_ObviousSafe(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "INSERT INTO audit_logs (id, action) VALUES ($1, $2)", "1", "LOGIN")
	return err
}

// N2: Legitimate Idiom — read-only SELECT from audit log table.
func N2_LegitimateIdiom(ctx context.Context, db DBExecutor) error {
	_, err := db.Query(ctx, "SELECT id, action FROM audit_logs WHERE id = $1", "1")
	return err
}

// N3: Unrelated API — non-database client method with same name (Exec).
type CacheStore struct{}

func (CacheStore) Exec(ctx context.Context, cmd string) (any, error) {
	return nil, nil
}

func N3_UnrelatedAPI(ctx context.Context, cache CacheStore) error {
	_, err := cache.Exec(ctx, "DELETE FROM audit_logs WHERE id = '1'")
	return err
}

// N4: Non-Audit Table Mutation — UPDATE on regular application tables is compliant.
func N4_NonAuditTable(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "UPDATE users SET name = $1 WHERE id = $2", "Alice", 1)
	return err
}

// N5: Static Constant Input — compile-time constant read-only count query on audit logs.
const CountAuditLogsQuery = "SELECT count(*) FROM audit_logs"

func N5_StaticConstant(ctx context.Context, db DBExecutor) error {
	_, err := db.Query(ctx, CountAuditLogsQuery)
	return err
}

// N6: Data Parameter Not SQL — query argument contains SQL syntax inside a string parameter.
func N6_DataParameterNotSQL(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "INSERT INTO audit_logs (id, payload) VALUES ($1, $2)", "1", "DELETE FROM audit_logs WHERE id = '1'")
	return err
}

// N7: Different Schema Audit Table — mutation on unconfigured schema does not collide with audit_logs.
func N7_DifferentSchemaAuditTable(ctx context.Context, db DBExecutor) error {
	_, err := db.Exec(ctx, "UPDATE evil_schema.audit_logs SET action = 'SAFE'")
	return err
}

