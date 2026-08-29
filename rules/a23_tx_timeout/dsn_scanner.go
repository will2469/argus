// Package a23_tx_timeout parses connection DSN strings for the transaction_timeout parameter.
package a23_tx_timeout

import (
	"net/url"
	"strings"
)

// CheckDSN inspects a DSN connection string for the transaction_timeout parameter.
func CheckDSN(dsn string) (hasTxTimeout bool, isZero bool) {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return false, false
	}

	// 1. URL query parameter parsing
	if u, err := url.Parse(trimmed); err == nil && u != nil {
		q := u.Query()
		if val := q.Get("transaction_timeout"); val != "" {
			if val == "0" || val == "0s" || val == "0ms" {
				return true, true
			}
			return true, false
		}
	}

	// 2. Key-value pair format (e.g. "host=localhost transaction_timeout=30000")
	for _, part := range strings.Fields(trimmed) {
		if strings.HasPrefix(strings.ToLower(part), "transaction_timeout=") {
			val := strings.TrimPrefix(strings.ToLower(part), "transaction_timeout=")
			val = strings.Trim(val, `"'`)
			if val == "0" || val == "0s" || val == "0ms" {
				return true, true
			}
			return true, false
		}
	}

	return false, false
}
