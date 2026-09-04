// Package a10_isolation_level maintains the registry and evaluation logic for critical database tables,
// enforcing schema-aware table identity and eliminating string-literal false positives.
package a10_isolation_level

import (
	"regexp"
	"strings"

	"github.com/will2469/argus/shared/sqlparser"
)

// TableRef represents a schema-qualified or unqualified database table.
type TableRef struct {
	Schema string
	Name   string
}

func (t TableRef) Key() string {
	if t.Schema != "" && t.Schema != "public" {
		return strings.ToLower(t.Schema + "." + t.Name)
	}
	return strings.ToLower(t.Name)
}

func (t TableRef) Matches(other TableRef) bool {
	if t.Name != other.Name {
		return false
	}
	tSchema := strings.ToLower(t.Schema)
	oSchema := strings.ToLower(other.Schema)

	// Both explicitly non-public schemas must match exactly
	if tSchema != "" && tSchema != "public" && oSchema != "" && oSchema != "public" {
		return tSchema == oSchema
	}
	// If one is explicitly non-public and the other is public or empty, they do NOT match
	if (tSchema != "" && tSchema != "public") || (oSchema != "" && oSchema != "public") {
		return false
	}
	return true
}

var defaultCriticalTables = map[string]bool{
	"balances":          true,
	"accounts":          true,
	"inventory":         true,
	"wallets":           true,
	"ledger":            true,
	"sequence_counters": true,
	"saldo":             true,
	"kuota":             true,
	"nomor_urut":        true,
	"rekening":          true,
}

// IsCriticalTable checks whether a table reference is in the critical table set, respecting schema boundaries.
func IsCriticalTable(ref TableRef, customTables []string) bool {
	tableName := strings.ToLower(ref.Name)
	schemaName := strings.ToLower(ref.Schema)

	// 1. Check custom configured tables
	for _, ct := range customTables {
		ct = strings.ToLower(strings.TrimSpace(ct))
		if strings.Contains(ct, ".") {
			parts := strings.SplitN(ct, ".", 2)
			if parts[0] == schemaName && parts[1] == tableName {
				return true
			}
		} else if ct == tableName {
			// If custom table has no schema specified, only match public/default schema
			if schemaName == "" || schemaName == "public" {
				return true
			}
		}
	}

	// 2. Default critical tables apply only to the default/public schema
	if schemaName != "" && schemaName != "public" {
		return false
	}

	return defaultCriticalTables[tableName]
}

// ExtractCriticalWriteTable checks if the given SQL statement modifies a critical table,
// returning the specific table reference.
func ExtractCriticalWriteTable(sql string, customTables []string) (TableRef, bool) {
	tree, err := sqlparser.Parse(sql)
	if err != nil {
		return heuristicExtractCriticalWrite(sql, customTables)
	}

	for _, stmt := range tree.Stmts {
		if stmt.Stmt == nil {
			continue
		}
		var ref TableRef
		if insert := stmt.Stmt.GetInsertStmt(); insert != nil && insert.Relation != nil {
			ref = TableRef{
				Schema: strings.ToLower(insert.Relation.Schemaname),
				Name:   strings.ToLower(insert.Relation.Relname),
			}
		} else if update := stmt.Stmt.GetUpdateStmt(); update != nil && update.Relation != nil {
			ref = TableRef{
				Schema: strings.ToLower(update.Relation.Schemaname),
				Name:   strings.ToLower(update.Relation.Relname),
			}
		} else if del := stmt.Stmt.GetDeleteStmt(); del != nil && del.Relation != nil {
			ref = TableRef{
				Schema: strings.ToLower(del.Relation.Schemaname),
				Name:   strings.ToLower(del.Relation.Relname),
			}
		}

		if ref.Name != "" && IsCriticalTable(ref, customTables) {
			return ref, true
		}
	}

	return TableRef{}, false
}

var (
	reStringLiteral = regexp.MustCompile(`'([^']|'')*'`)
	reDML           = regexp.MustCompile(`(?i)(?:UPDATE|INSERT\s+INTO|DELETE\s+FROM)\s+([a-zA-Z0-9_.]+)`)
)

func heuristicExtractCriticalWrite(sql string, customTables []string) (TableRef, bool) {
	// Strip string literals to avoid false positives on comments or audit messages
	cleanSQL := reStringLiteral.ReplaceAllString(sql, "''")

	matches := reDML.FindStringSubmatch(cleanSQL)
	if len(matches) > 1 {
		rawTarget := strings.ToLower(strings.Trim(matches[1], "\""))
		parts := strings.Split(rawTarget, ".")
		var ref TableRef
		if len(parts) == 2 {
			ref = TableRef{Schema: parts[0], Name: parts[1]}
		} else {
			ref = TableRef{Name: parts[0]}
		}
		if IsCriticalTable(ref, customTables) {
			return ref, true
		}
	}

	return TableRef{}, false
}
