// Package a19_unbounded_limit inspects SQL AST nodes for unbounded queries on high-cardinality tables.
package a19_unbounded_limit

import (
	"regexp"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// CheckUnboundedQuery inspects an SQL query to determine if it scans a high-cardinality table without LIMIT.
func CheckUnboundedQuery(sql string, tableMap map[string]bool, keyColumnMap map[string]bool) (bool, string) {
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
			if unbounded, table := checkNodeForUnbounded(stmt.Stmt.Node, tableMap, keyColumnMap, false); unbounded {
				return true, table
			}
		}
		return false, ""
	}

	// Regex fallback if query parsing fails
	for table := range tableMap {
		pattern := `(?i)\bFROM\s+(?:\w+\.)?` + regexp.QuoteMeta(table) + `\b`
		if matched, _ := regexp.MatchString(pattern, trimmed); matched {
			if !IsExemptRegex(trimmed) {
				return true, table
			}
		}
	}

	return false, ""
}

func checkNodeForUnbounded(node any, tableMap map[string]bool, keyColumnMap map[string]bool, insideExists bool) (bool, string) {
	if node == nil {
		return false, ""
	}
	if n, ok := node.(*pg_query.Node_SelectStmt); ok {
		return checkSelectStmt(n.SelectStmt, tableMap, keyColumnMap, insideExists)
	}
	return false, ""
}

func checkSelectStmt(sel *pg_query.SelectStmt, tableMap map[string]bool, keyColumnMap map[string]bool, insideExists bool) (bool, string) {
	if sel == nil {
		return false, ""
	}

	// 1. If inside an EXISTS subquery, row count is inherently bounded
	if insideExists {
		return false, ""
	}

	// 2. Check if current query is bounded by LIMIT / FETCH FIRST
	if sel.LimitCount != nil || sel.LimitOffset != nil {
		return false, ""
	}

	// 3. Check if query qualifies for scalar aggregate exemption
	if IsScalarAggregate(sel) {
		return false, ""
	}

	// 4. Check if query qualifies for point lookup exemption
	if IsPointLookup(sel, keyColumnMap) {
		return false, ""
	}

	// 5. Inspect CTEs in WithClause
	if sel.WithClause != nil {
		for _, cteNode := range sel.WithClause.Ctes {
			if cte := cteNode.GetCommonTableExpr(); cte != nil && cte.Ctequery != nil {
				if unbounded, table := checkNodeForUnbounded(cte.Ctequery.Node, tableMap, keyColumnMap, false); unbounded {
					return true, table
				}
			}
		}
	}

	// 6. Inspect FromClause subqueries
	for _, from := range sel.FromClause {
		if sub := from.GetRangeSubselect(); sub != nil && sub.Subquery != nil {
			if unbounded, table := checkNodeForUnbounded(sub.Subquery.Node, tableMap, keyColumnMap, false); unbounded {
				return true, table
			}
		}
	}

	// 7. Inspect WhereClause subqueries
	if sel.WhereClause != nil {
		if unbounded, table := checkExprForUnbounded(sel.WhereClause, tableMap, keyColumnMap); unbounded {
			return true, table
		}
	}

	// 8. Inspect UNION / INTERSECT / EXCEPT branches
	if sel.Larg != nil {
		if unbounded, table := checkSelectStmt(sel.Larg, tableMap, keyColumnMap, false); unbounded {
			return true, table
		}
	}
	if sel.Rarg != nil {
		if unbounded, table := checkSelectStmt(sel.Rarg, tableMap, keyColumnMap, false); unbounded {
			return true, table
		}
	}

	// 9. Check if current SELECT targets any high-cardinality table
	tables := extractTablesFromNodes(sel.FromClause, keyColumnMap)
	var highCardTable string
	for _, tbl := range tables {
		if IsHighCardinalityTable(tbl, tableMap) {
			highCardTable = tbl
			break
		}
	}

	if highCardTable == "" {
		return false, ""
	}

	return true, highCardTable
}

func checkExprForUnbounded(node *pg_query.Node, tableMap map[string]bool, keyColumnMap map[string]bool) (bool, string) {
	if node == nil {
		return false, ""
	}

	if subLink := node.GetSubLink(); subLink != nil {
		isExists := IsSublinkExists(subLink)
		if subLink.Subselect != nil {
			return checkNodeForUnbounded(subLink.Subselect.Node, tableMap, keyColumnMap, isExists)
		}
	}

	if boolExpr := node.GetBoolExpr(); boolExpr != nil {
		for _, arg := range boolExpr.Args {
			if unbounded, table := checkExprForUnbounded(arg, tableMap, keyColumnMap); unbounded {
				return true, table
			}
		}
	}

	return false, ""
}

func extractTablesFromNodes(fromNodes []*pg_query.Node, keyColumnMap map[string]bool) []string {
	var tables []string
	for _, node := range fromNodes {
		extractTablesFromNode(node, &tables, keyColumnMap)
	}
	return tables
}

func extractTablesFromNode(node *pg_query.Node, tables *[]string, keyColumnMap map[string]bool) {
	if node == nil {
		return
	}

	if rangeVar := node.GetRangeVar(); rangeVar != nil {
		if rangeVar.Relname != "" {
			*tables = append(*tables, rangeVar.Relname)
		}
		return
	}

	if joinExpr := node.GetJoinExpr(); joinExpr != nil {
		extractTablesFromNode(joinExpr.Larg, tables, keyColumnMap)
		if !checkExprForPointLookup(joinExpr.Quals, keyColumnMap) {
			extractTablesFromNode(joinExpr.Rarg, tables, keyColumnMap)
		}
		return
	}
}
