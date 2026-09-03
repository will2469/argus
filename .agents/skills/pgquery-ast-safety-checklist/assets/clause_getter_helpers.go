package astsafety

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// ExtractWhereClause safely extracts the WHERE clause from any SQL statement node.
func ExtractWhereClause(stmt *pg_query.Node) *pg_query.Node {
	if stmt == nil {
		return nil
	}

	switch s := stmt.Node.(type) {
	case *pg_query.Node_SelectStmt:
		if s.SelectStmt != nil {
			return s.SelectStmt.WhereClause
		}
	case *pg_query.Node_UpdateStmt:
		if s.UpdateStmt != nil {
			return s.UpdateStmt.WhereClause
		}
	case *pg_query.Node_DeleteStmt:
		if s.DeleteStmt != nil {
			return s.DeleteStmt.WhereClause
		}
	case *pg_query.Node_IndexStmt:
		if s.IndexStmt != nil {
			return s.IndexStmt.WhereClause // Partial index predicate
		}
	}

	return nil
}

// SortByInfo represents an ORDER BY column and direction.
type SortByInfo struct {
	TargetNode *pg_query.Node
	IsDesc     bool
	NullsFirst bool
}

// ExtractSortClauses extracts SortBy items from a SelectStmt.
func ExtractSortClauses(sel *pg_query.SelectStmt) []SortByInfo {
	if sel == nil || len(sel.SortClause) == 0 {
		return nil
	}

	var results []SortByInfo
	for _, item := range sel.SortClause {
		sortBy := item.GetSortBy()
		if sortBy == nil {
			continue
		}

		info := SortByInfo{
			TargetNode: sortBy.Node,
			IsDesc:     sortBy.SortbyDir == pg_query.SortByDir_SORTBY_DESC,
			NullsFirst: sortBy.SortbyNulls == pg_query.SortByNulls_SORTBY_NULLS_FIRST,
		}
		results = append(results, info)
	}

	return results
}

// LockInfo details row locking clauses (FOR UPDATE / FOR SHARE / NOWAIT / SKIP LOCKED).
type LockInfo struct {
	HasLocking bool
	Strength   pg_query.LockClauseStrength
	WaitPolicy pg_query.LockWaitPolicy
}

// ExtractLockingInfo extracts concurrency locking options from SelectStmt.
func ExtractLockingInfo(sel *pg_query.SelectStmt) LockInfo {
	if sel == nil || len(sel.LockingClause) == 0 {
		return LockInfo{HasLocking: false}
	}

	for _, item := range sel.LockingClause {
		clause := item.GetLockingClause()
		if clause == nil {
			continue
		}
		return LockInfo{
			HasLocking: true,
			Strength:   clause.Strength,
			WaitPolicy: clause.WaitPolicy,
		}
	}

	return LockInfo{HasLocking: false}
}
