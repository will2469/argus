// Package a28_constraint_lock provides table creation helpers for constraint addition checks.
package a28_constraint_lock

import (
	pgquery "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/sqlparser"
)

// CollectCreatedTables delegates to the shared sqlparser table collector.
func CollectCreatedTables(tree *pgquery.ParseResult) map[string]bool {
	return sqlparser.CollectCreatedTables(tree)
}
