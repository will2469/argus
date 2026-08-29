// Package a14_select_star defines strict whitelists and exceptions for wildcard queries.
package a14_select_star

import (
	"regexp"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var (
	countStarRegex  = regexp.MustCompile(`(?i)\bCOUNT\s*\(\s*(?:DISTINCT\s+)?\*\s*\)`)
	existsStarRegex = regexp.MustCompile(`(?i)\bEXISTS\s*\(\s*SELECT\s+(?:(?:\w+\.)?\*|1)\s+FROM\b`)
)

// IsExemptRegex checks if non-parsable query qualifies under regex exception whitelist.
func IsExemptRegex(sql string) bool {
	return countStarRegex.MatchString(sql) || existsStarRegex.MatchString(sql)
}

// IsSublinkExists checks if a subquery node represents an EXISTS(...) construct.
func IsSublinkExists(subLink *pg_query.SubLink) bool {
	if subLink == nil {
		return false
	}
	return subLink.SubLinkType == pg_query.SubLinkType_EXISTS_SUBLINK
}
