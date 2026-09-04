// Package a06_runtime_ddl recognizes DDL statement command tokens and filters out compliant DML.
package a06_runtime_ddl

import (
	"strings"
)

// MatchDDLCommand inspects a SQL string or fragment and returns the detected DDL operation name,
// or an empty string if the statement is DML or non-DDL.
func MatchDDLCommand(s string) string {
	stmts := strings.Split(s, ";")
	for _, stmt := range stmts {
		clean := cleanSQLFragment(stmt)
		if clean == "" {
			continue
		}
		upper := strings.ToUpper(clean)

		// Fast path: standard DML queries are compliant
		if startsWithDML(upper) {
			continue
		}

		if op := matchDDLPrefix(upper); op != "" {
			return op
		}
	}
	return ""
}

func startsWithDML(upper string) bool {
	fields := strings.Fields(upper)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "EXPLAIN", "COPY", "SET", "SHOW", "BEGIN", "COMMIT", "ROLLBACK":
		return true
	}
	return false
}

func matchDDLPrefix(upper string) string {
	fields := strings.Fields(upper)
	if len(fields) == 0 {
		return ""
	}

	switch fields[0] {
	case "CREATE":
		if len(fields) >= 2 {
			switch fields[1] {
			case "TABLE", "TEMP", "TEMPORARY", "UNLOGGED":
				return "CREATE TABLE"
			case "INDEX", "UNIQUE":
				return "CREATE INDEX"
			case "SCHEMA":
				return "CREATE SCHEMA"
			case "SEQUENCE":
				return "CREATE SEQUENCE"
			case "VIEW", "MATERIALIZED":
				return "CREATE VIEW"
			case "TRIGGER":
				return "CREATE TRIGGER"
			case "FUNCTION":
				return "CREATE FUNCTION"
			case "PROCEDURE":
				return "CREATE PROCEDURE"
			case "EXTENSION":
				return "CREATE EXTENSION"
			case "TYPE", "DOMAIN":
				return "CREATE TYPE"
			case "ROLE", "USER":
				return "GRANT ROLE"
			case "OR":
				if len(fields) >= 4 && fields[2] == "REPLACE" {
					switch fields[3] {
					case "VIEW":
						return "CREATE VIEW"
					case "TRIGGER":
						return "CREATE TRIGGER"
					case "FUNCTION":
						return "CREATE FUNCTION"
					case "PROCEDURE":
						return "CREATE PROCEDURE"
					}
				}
				return "CREATE"
			}
		}
		return "CREATE TABLE"

	case "DROP":
		return "DROP"

	case "ALTER":
		if len(fields) >= 2 {
			switch fields[1] {
			case "TABLE":
				return "ALTER TABLE"
			case "SEQUENCE":
				return "ALTER SEQUENCE"
			case "INDEX":
				return "ALTER INDEX"
			case "SCHEMA":
				return "ALTER SCHEMA"
			case "VIEW":
				return "ALTER VIEW"
			}
		}
		return "ALTER TABLE"

	case "TRUNCATE":
		return "TRUNCATE"

	case "GRANT", "REVOKE":
		return "GRANT/REVOKE"

	case "COMMENT":
		if len(fields) >= 2 && fields[1] == "ON" {
			return "COMMENT"
		}

	case "RENAME":
		return "RENAME"
	}

	return ""
}

func cleanSQLFragment(s string) string {
	trimmed := strings.TrimSpace(s)
	for strings.HasPrefix(trimmed, "--") {
		idx := strings.Index(trimmed, "\n")
		if idx == -1 {
			return ""
		}
		trimmed = strings.TrimSpace(trimmed[idx+1:])
	}
	for strings.HasPrefix(trimmed, "/*") {
		idx := strings.Index(trimmed, "*/")
		if idx == -1 {
			return ""
		}
		trimmed = strings.TrimSpace(trimmed[idx+2:])
	}
	return trimmed
}
