// Package sqlparser provides helpers and caching for pg_query_go PostgreSQL AST parsing.
package sqlparser

import (
	"strings"
	"sync"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var (
	cacheMu  sync.RWMutex
	astCache = make(map[string]*pg_query.ParseResult)
)

const maxCacheEntries = 1000

// Parse parses a SQL query into a PostgreSQL AST with caching for speed.
func Parse(query string) (*pg_query.ParseResult, error) {
	cacheMu.RLock()
	if cached, ok := astCache[query]; ok {
		cacheMu.RUnlock()
		return cached, nil
	}
	cacheMu.RUnlock()

	res, err := pg_query.Parse(query)
	if err != nil {
		return nil, err
	}

	cacheMu.Lock()
	if len(astCache) < maxCacheEntries {
		astCache[query] = res
	}
	cacheMu.Unlock()

	return res, nil
}

// ExtractFromTableNames extracts all table names referenced in a SelectStmt's FromClause.
func ExtractFromTableNames(stmt *pg_query.SelectStmt) []string {
	if stmt == nil || stmt.FromClause == nil {
		return nil
	}
	var tables []string
	for _, item := range stmt.FromClause {
		if rangeVar := item.GetRangeVar(); rangeVar != nil {
			if rangeVar.Relname != "" {
				tables = append(tables, rangeVar.Relname)
			}
		}
		if joinExpr := item.GetJoinExpr(); joinExpr != nil {
			if joinExpr.Larg != nil && joinExpr.Larg.GetRangeVar() != nil {
				tables = append(tables, joinExpr.Larg.GetRangeVar().Relname)
			}
			if joinExpr.Rarg != nil && joinExpr.Rarg.GetRangeVar() != nil {
				tables = append(tables, joinExpr.Rarg.GetRangeVar().Relname)
			}
		}
	}
	return tables
}

// HasSelectStar checks if a SelectStmt projects a wildcard (*) or alias.*.
// Ignores aggregate functions like COUNT(*).
func HasSelectStar(stmt *pg_query.SelectStmt) bool {
	if stmt == nil || stmt.TargetList == nil {
		return false
	}
	for _, target := range stmt.TargetList {
		resTarget := target.GetResTarget()
		if resTarget == nil || resTarget.Val == nil {
			continue
		}

		// Check ColumnRef with wildcard (*)
		if colRef := resTarget.Val.GetColumnRef(); colRef != nil {
			for _, field := range colRef.Fields {
				if field.GetAStar() != nil {
					return true
				}
			}
		}
	}
	return false
}

// ExtractLockingInfo inspects a SelectStmt's LockingClause.
// Returns hasLocking, isForUpdate, isSkipLocked, isNoWait.
func ExtractLockingInfo(stmt *pg_query.SelectStmt) (hasLocking bool, isForUpdate bool, isSkipLocked bool, isNoWait bool) {
	if stmt == nil || stmt.LockingClause == nil || len(stmt.LockingClause) == 0 {
		return false, false, false, false
	}
	hasLocking = true
	for _, item := range stmt.LockingClause {
		clause := item.GetLockingClause()
		if clause == nil {
			continue
		}
		if clause.Strength == pg_query.LockClauseStrength_LCS_FORUPDATE ||
			clause.Strength == pg_query.LockClauseStrength_LCS_FORNOKEYUPDATE {
			isForUpdate = true
		}
		switch clause.WaitPolicy {
		case pg_query.LockWaitPolicy_LockWaitSkip:
			isSkipLocked = true
		case pg_query.LockWaitPolicy_LockWaitError:
			isNoWait = true
		}
	}
	return
}

// CollectCreatedTables extracts table names (in lowercase) defined via CREATE TABLE in an AST parse tree.
func CollectCreatedTables(tree *pg_query.ParseResult) map[string]bool {
	createdTables := make(map[string]bool)
	if tree == nil {
		return createdTables
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}
		if createStmt := rawStmt.Stmt.GetCreateStmt(); createStmt != nil {
			if createStmt.Relation != nil && createStmt.Relation.Relname != "" {
				createdTables[strings.ToLower(createStmt.Relation.Relname)] = true
			}
		}
	}

	return createdTables
}
