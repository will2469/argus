// Package a21_row_lock inspects SQL AST nodes for blocking row locks without SKIP LOCKED or NOWAIT.
package a21_row_lock

import (
	"regexp"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var (
	forUpdateRegex   = regexp.MustCompile(`(?i)\bFOR\s+(?:UPDATE|NO\s+KEY\s+UPDATE|SHARE|KEY\s+SHARE)\b`)
	skipLockedRegex  = regexp.MustCompile(`(?i)\bSKIP\s+LOCKED\b`)
	nowaitRegex      = regexp.MustCompile(`(?i)\bNOWAIT\b`)
	pointLookupRegex = regexp.MustCompile(`(?i)\bWHERE\s+.*(?:id|uuid|pk|[a-zA-Z0-9_]+_id|[a-zA-Z0-9_]+_hash)\s*=\s*(?:\$\d+|\?)`)
)

// CheckLockingQuery inspects an SQL query for blocking multi-row lock clauses without SKIP LOCKED or NOWAIT.
func CheckLockingQuery(sql string, keyColumnMap map[string]bool) (bool, string) {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return false, ""
	}

	res, err := pg_query.Parse(trimmed)
	if err == nil && res != nil {
		for _, stmt := range res.Stmts {
			if stmt == nil || stmt.Stmt == nil {
				continue
			}
			if violating, reason := checkNodeForBlockingLock(stmt.Stmt.Node, keyColumnMap); violating {
				return true, reason
			}
		}
		return false, ""
	}

	// Regex fallback if query parsing fails
	if forUpdateRegex.MatchString(trimmed) {
		if !skipLockedRegex.MatchString(trimmed) && !nowaitRegex.MatchString(trimmed) {
			if !pointLookupRegex.MatchString(trimmed) {
				return true, "blocking row-level lock (FOR UPDATE / FOR NO KEY UPDATE) without SKIP LOCKED or NOWAIT; risk of worker lock convoy (CWE-662, CWE-833)"
			}
		}
	}

	return false, ""
}

func checkNodeForBlockingLock(node any, keyColumnMap map[string]bool) (bool, string) {
	if node == nil {
		return false, ""
	}
	if n, ok := node.(*pg_query.Node_SelectStmt); ok {
		return checkSelectStmt(n.SelectStmt, keyColumnMap)
	}
	return false, ""
}

func checkSelectStmt(sel *pg_query.SelectStmt, keyColumnMap map[string]bool) (bool, string) {
	if sel == nil {
		return false, ""
	}

	if len(sel.LockingClause) > 0 {
		for _, lockNode := range sel.LockingClause {
			lc := lockNode.GetLockingClause()
			if lc == nil {
				continue
			}

			// Only evaluate strong row-locking strengths
			switch lc.Strength {
			case pg_query.LockClauseStrength_LCS_FORUPDATE,
				pg_query.LockClauseStrength_LCS_FORNOKEYUPDATE:

				// If policy is SKIP LOCKED or NOWAIT, it's non-blocking and safe
				if lc.WaitPolicy == pg_query.LockWaitPolicy_LockWaitSkip ||
					lc.WaitPolicy == pg_query.LockWaitPolicy_LockWaitError {
					continue
				}

				// If policy is LockWaitBlock (default blocking):
				// Allow single-entity point lookup on primary key
				if IsPointLookup(sel, keyColumnMap) {
					continue
				}

				return true, "blocking row-level lock (FOR UPDATE / FOR NO KEY UPDATE) without SKIP LOCKED or NOWAIT; risk of worker lock convoy (CWE-662, CWE-833)"
			}
		}
	}

	// Check CTEs
	if sel.WithClause != nil {
		for _, cteNode := range sel.WithClause.Ctes {
			if cte := cteNode.GetCommonTableExpr(); cte != nil && cte.Ctequery != nil {
				if violating, reason := checkNodeForBlockingLock(cte.Ctequery.Node, keyColumnMap); violating {
					return true, reason
				}
			}
		}
	}

	// Check UNION / branches
	if sel.Larg != nil {
		if violating, reason := checkSelectStmt(sel.Larg, keyColumnMap); violating {
			return true, reason
		}
	}
	if sel.Rarg != nil {
		if violating, reason := checkSelectStmt(sel.Rarg, keyColumnMap); violating {
			return true, reason
		}
	}

	return false, ""
}
