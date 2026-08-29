// Package a27_concurrent_index provides table creation helpers for concurrent index checks.
package a27_concurrent_index

import (
	pgquery "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/sqlparser"
)

// CollectCreatedTables delegates to the shared sqlparser table collector.
func CollectCreatedTables(tree *pgquery.ParseResult) map[string]bool {
	return sqlparser.CollectCreatedTables(tree)
}
