// Package a30_timestamptz provides type AST inspection functions to detect bare TIMESTAMP types.
package a30_timestamptz

import (
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"
)

// IsBareTimestamp returns true if the given TypeName AST node represents TIMESTAMP without time zone.
func IsBareTimestamp(typeName *pgquery.TypeName) bool {
	if typeName == nil || len(typeName.Names) == 0 {
		return false
	}
	var parts []string
	for _, n := range typeName.Names {
		if s := n.GetString_(); s != nil {
			parts = append(parts, strings.ToLower(s.Sval))
		}
	}
	fullName := strings.Join(parts, ".")
	// In pg_query_go:
	// "timestamp" or "pg_catalog.timestamp" -> bare TIMESTAMP without time zone (forbidden)
	// "timestamptz" or "pg_catalog.timestamptz" -> TIMESTAMP WITH TIME ZONE (safe)
	return fullName == "timestamp" || fullName == "pg_catalog.timestamp"
}
