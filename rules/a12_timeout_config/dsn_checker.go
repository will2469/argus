package a12_timeout_config

import (
	"net/url"
	"strings"
)

// DSNCheckResult holds missing and invalid timeout parameters found in a DSN.
type DSNCheckResult struct {
	Missing []string
	Zero    []string
}

// CheckDSN evaluates a PostgreSQL connection string for required timeout parameters.
func CheckDSN(dsn string) DSNCheckResult {
	var result DSNCheckResult

	u, err := url.Parse(dsn)
	var q url.Values
	if err == nil {
		q = u.Query()
	}

	checkParam := func(name string, aliases ...string) {
		val := ""
		found := false

		if q != nil {
			if v := q.Get(name); v != "" {
				val = v
				found = true
			} else {
				for _, alias := range aliases {
					if v := q.Get(alias); v != "" {
						val = v
						found = true
						break
					}
				}
			}
		}

		if !found {
			// Fallback text check for key=value pairs in non-URL DSNs
			for _, key := range append([]string{name}, aliases...) {
				if v, ok := findKVParam(dsn, key); ok {
					val = v
					found = true
					break
				}
			}
		}

		if !found {
			result.Missing = append(result.Missing, name)
		} else if isZeroValue(val) {
			result.Zero = append(result.Zero, name)
		}
	}

	checkParam("statement_timeout")
	checkParam("lock_timeout")
	checkParam("idle_in_transaction_session_timeout", "idle_in_transaction")

	return result
}

func findKVParam(dsn, key string) (string, bool) {
	lowerDSN := strings.ToLower(dsn)
	target := strings.ToLower(key) + "="
	idx := strings.Index(lowerDSN, target)
	if idx == -1 {
		return "", false
	}
	start := idx + len(target)
	rest := dsn[start:]
	end := strings.IndexAny(rest, " &;\n\r\t")
	if end == -1 {
		return rest, true
	}
	return rest[:end], true
}

func isZeroValue(val string) bool {
	v := strings.Trim(val, `"' `)
	v = strings.ToLower(v)
	return v == "0" || v == "0s" || v == "0ms" || v == "0m"
}
