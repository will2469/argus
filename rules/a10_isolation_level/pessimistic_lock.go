// Package a10_isolation_level detects pessimistic row locking and advisory lock protection.
package a10_isolation_level

import (
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/sqlparser"
)

// HasPessimisticRowLock checks if a query includes SELECT ... FOR UPDATE or FOR NO KEY UPDATE.
func HasPessimisticRowLock(sql string) bool {
	tree, err := sqlparser.Parse(sql)
	if err != nil {
		upper := strings.ToUpper(sql)
		return strings.Contains(upper, "FOR UPDATE") || strings.Contains(upper, "FOR NO KEY UPDATE")
	}

	for _, stmt := range tree.Stmts {
		if stmt.Stmt == nil {
			continue
		}
		if sel := stmt.Stmt.GetSelectStmt(); sel != nil {
			if len(sel.LockingClause) > 0 {
				for _, lc := range sel.LockingClause {
					if clause := lc.GetLockingClause(); clause != nil {
						if clause.Strength == pg_query.LockClauseStrength_LCS_FORUPDATE ||
							clause.Strength == pg_query.LockClauseStrength_LCS_FORNOKEYUPDATE {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// HasAdvisoryLockCall checks if a SQL query acquires an advisory transaction lock.
func HasAdvisoryLockCall(sql string) bool {
	lower := strings.ToLower(sql)
	return strings.Contains(lower, "pg_advisory_xact_lock") || strings.Contains(lower, "pg_try_advisory_xact_lock")
}
