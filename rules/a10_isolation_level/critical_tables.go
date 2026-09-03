// Package a10_isolation_level maintains the registry and evaluation logic for critical database tables.
package a10_isolation_level

import (
	"strings"

	"github.com/will2469/argus/shared/sqlparser"
)

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

// IsCriticalTable checks whether a table name is in the critical table set.
func IsCriticalTable(name string, customTables []string) bool {
	lower := strings.ToLower(strings.Trim(name, "\""))
	if defaultCriticalTables[lower] {
		return true
	}
	for _, ct := range customTables {
		if strings.EqualFold(lower, ct) {
			return true
		}
	}
	return false
}

// IsCriticalTableWrite checks if the given SQL statement modifies a critical table.
func IsCriticalTableWrite(sql string, customTables []string) bool {
	tree, err := sqlparser.Parse(sql)
	if err != nil {
		return heuristicCriticalWrite(sql, customTables)
	}

	for _, stmt := range tree.Stmts {
		if stmt.Stmt == nil {
			continue
		}
		var tableName string
		if insert := stmt.Stmt.GetInsertStmt(); insert != nil && insert.Relation != nil {
			tableName = insert.Relation.Relname
		} else if update := stmt.Stmt.GetUpdateStmt(); update != nil && update.Relation != nil {
			tableName = update.Relation.Relname
		} else if del := stmt.Stmt.GetDeleteStmt(); del != nil && del.Relation != nil {
			tableName = del.Relation.Relname
		}

		if tableName != "" && IsCriticalTable(tableName, customTables) {
			return true
		}
	}

	return false
}

func heuristicCriticalWrite(sql string, customTables []string) bool {
	upper := strings.ToUpper(sql)
	if !strings.Contains(upper, "INSERT") && !strings.Contains(upper, "UPDATE") && !strings.Contains(upper, "DELETE") {
		return false
	}
	for t := range defaultCriticalTables {
		if strings.Contains(strings.ToLower(sql), t) {
			return true
		}
	}
	for _, ct := range customTables {
		if strings.Contains(strings.ToLower(sql), strings.ToLower(ct)) {
			return true
		}
	}
	return false
}
