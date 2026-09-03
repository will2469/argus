package assets

import (
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"
)

// ParseSQLOrFragment attempts to parse an SQL string with pg_query_go.
// If direct parsing fails because the SQL string is an expression or clause fragment
// (e.g. "WHERE ...", "AND ...", or "col LIKE $1"), it attempts wrapping with boilerplate queries.
// INVARIANT: Uses "SELECT 1 FROM __argus_dummy__" (never "SELECT *") to avoid triggering A14.
func ParseSQLOrFragment(sql string) (*pgquery.ParseResult, error) {
	result, err := pgquery.Parse(sql)
	if err == nil {
		return result, nil
	}

	trimmed := strings.TrimSpace(sql)
	upperTrimmed := strings.ToUpper(trimmed)

	var wrappers []string
	if strings.HasPrefix(upperTrimmed, "WHERE") {
		wrappers = append(wrappers, "SELECT 1 FROM __argus_dummy__ "+trimmed)
	} else if strings.HasPrefix(upperTrimmed, "AND") || strings.HasPrefix(upperTrimmed, "OR") {
		wrappers = append(wrappers, "SELECT 1 FROM __argus_dummy__ WHERE 1=1 "+trimmed)
	} else {
		wrappers = append(wrappers, "SELECT 1 FROM __argus_dummy__ WHERE "+trimmed)
	}

	for _, wrapped := range wrappers {
		if res, wrapErr := pgquery.Parse(wrapped); wrapErr == nil {
			return res, nil
		}
	}

	return nil, err
}
