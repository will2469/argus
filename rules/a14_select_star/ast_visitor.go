// Package a14_select_star traverses SQL AST nodes to identify forbidden SELECT * projections.
package a14_select_star

import (
	"regexp"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var selectStarRegex = regexp.MustCompile(`(?i)\bSELECT\s+(?:DISTINCT\s+)?(?:\w+\.)?\*\s+(?:FROM|\n\s*FROM)\b`)

// HasForbiddenSelectStar checks if SQL string contains an unpermitted SELECT * or alias.*.
// Exempts COUNT(*) and EXISTS (SELECT * ...).
func HasForbiddenSelectStar(sql string) bool {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return false
	}

	res, err := pg_query.Parse(trimmed)
	if err == nil && res != nil {
		return hasStarAST(res)
	}

	// Regex fallback if query cannot be parsed (e.g. fragmented dynamic query)
	if selectStarRegex.MatchString(trimmed) {
		return !IsExemptRegex(trimmed)
	}
	return false
}

func hasStarAST(res *pg_query.ParseResult) bool {
	for _, stmt := range res.Stmts {
		if stmt == nil || stmt.Stmt == nil {
			continue
		}
		if checkNodeForSelectStar(stmt.Stmt.Node, false) {
			return true
		}
	}
	return false
}

func checkSelectStmt(sel *pg_query.SelectStmt, insideExists bool) bool {
	if sel == nil {
		return false
	}

	// 1. Inspect TargetList columns (unless inside an EXISTS subquery)
	if !insideExists {
		for _, targetNode := range sel.TargetList {
			if resTarget := targetNode.GetResTarget(); resTarget != nil && resTarget.Val != nil {
				if colRef := resTarget.Val.GetColumnRef(); colRef != nil {
					for _, field := range colRef.Fields {
						if field.GetAStar() != nil {
							return true
						}
					}
				}
			}
		}
	}

	// 2. Inspect FromClause subqueries
	for _, from := range sel.FromClause {
		if sub := from.GetRangeSubselect(); sub != nil && sub.Subquery != nil {
			if checkNodeForSelectStar(sub.Subquery.Node, false) {
				return true
			}
		}
	}

	// 3. Inspect WithClause (CTEs)
	if sel.WithClause != nil {
		for _, cteNode := range sel.WithClause.Ctes {
			if cte := cteNode.GetCommonTableExpr(); cte != nil && cte.Ctequery != nil {
				if checkNodeForSelectStar(cte.Ctequery.Node, false) {
					return true
				}
			}
		}
	}

	// 4. Inspect WhereClause for subqueries (identifying EXISTS subqueries)
	if sel.WhereClause != nil {
		if checkExprForSelectStar(sel.WhereClause) {
			return true
		}
	}

	// 5. Inspect Larg/Rarg (UNION/INTERSECT/EXCEPT)
	if sel.Larg != nil && checkSelectStmt(sel.Larg, false) {
		return true
	}
	if sel.Rarg != nil && checkSelectStmt(sel.Rarg, false) {
		return true
	}

	return false
}

func checkNodeForSelectStar(node any, insideExists bool) bool {
	if node == nil {
		return false
	}
	if n, ok := node.(*pg_query.Node_SelectStmt); ok {
		return checkSelectStmt(n.SelectStmt, insideExists)
	}
	return false
}

func checkExprForSelectStar(node *pg_query.Node) bool {
	if node == nil {
		return false
	}

	if subLink := node.GetSubLink(); subLink != nil {
		isExists := IsSublinkExists(subLink)
		if subLink.Subselect != nil {
			return checkNodeForSelectStar(subLink.Subselect.Node, isExists)
		}
	}

	if boolExpr := node.GetBoolExpr(); boolExpr != nil {
		for _, arg := range boolExpr.Args {
			if checkExprForSelectStar(arg) {
				return true
			}
		}
	}

	return false
}
