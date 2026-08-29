// Package a19_unbounded_limit detects valid query exemptions (aggregates, point lookups).
package a19_unbounded_limit

import (
	"regexp"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

var (
	scalarCountRegex = regexp.MustCompile(`(?i)\bSELECT\s+COUNT\s*\([^)]*\)\s+FROM\b`)
	groupByRegex     = regexp.MustCompile(`(?i)\bGROUP\s+BY\b`)
	pointLookupRegex = regexp.MustCompile(`(?i)\bWHERE\s+.*(?:id|uuid|pk|[a-zA-Z0-9_]+_id|[a-zA-Z0-9_]+_hash)\s*(?:=|\bIN\b|\bANY\b)`)
	limitRegex       = regexp.MustCompile(`(?i)\b(?:LIMIT|FETCH\s+FIRST)\b`)
)

// IsScalarAggregate checks if a SELECT query projects only scalar aggregation functions without GROUP BY.
func IsScalarAggregate(sel *pg_query.SelectStmt) bool {
	if sel == nil {
		return false
	}
	if len(sel.GroupClause) > 0 {
		return false
	}
	if len(sel.TargetList) == 0 {
		return false
	}

	for _, targetNode := range sel.TargetList {
		resTarget := targetNode.GetResTarget()
		if resTarget == nil || resTarget.Val == nil {
			return false
		}

		funcCall := resTarget.Val.GetFuncCall()
		if funcCall == nil {
			return false
		}

		funcName := ""
		for _, nameNode := range funcCall.Funcname {
			if str := nameNode.GetString_(); str != nil {
				funcName = strings.ToUpper(str.Sval)
			}
		}

		switch funcName {
		case "COUNT", "SUM", "AVG", "MIN", "MAX":
			// Valid scalar aggregate
		default:
			return false
		}
	}

	return true
}

// IsPointLookup checks if the WHERE clause contains an equality check on a primary or key column.
func IsPointLookup(sel *pg_query.SelectStmt, keyColumnMap map[string]bool) bool {
	if sel == nil || sel.WhereClause == nil {
		return false
	}
	return checkExprForPointLookup(sel.WhereClause, keyColumnMap)
}

func checkExprForPointLookup(node *pg_query.Node, keyColumnMap map[string]bool) bool {
	if node == nil {
		return false
	}

	if aExpr := node.GetAExpr(); aExpr != nil {
		switch aExpr.Kind {
		case pg_query.A_Expr_Kind_AEXPR_OP, pg_query.A_Expr_Kind_AEXPR_OP_ANY, pg_query.A_Expr_Kind_AEXPR_IN:
			opName := ""
			for _, nameNode := range aExpr.Name {
				if str := nameNode.GetString_(); str != nil {
					opName = str.Sval
				}
			}
			if opName == "=" || opName == "" || aExpr.Kind == pg_query.A_Expr_Kind_AEXPR_IN || aExpr.Kind == pg_query.A_Expr_Kind_AEXPR_OP_ANY {
				if isKeyColumnRef(aExpr.Lexpr, keyColumnMap) || isKeyColumnRef(aExpr.Rexpr, keyColumnMap) {
					return true
				}
			}
		}
	}

	if boolExpr := node.GetBoolExpr(); boolExpr != nil {
		if boolExpr.Boolop == pg_query.BoolExprType_AND_EXPR {
			for _, arg := range boolExpr.Args {
				if checkExprForPointLookup(arg, keyColumnMap) {
					return true
				}
			}
		}
	}

	return false
}

func isKeyColumnRef(node *pg_query.Node, keyColumnMap map[string]bool) bool {
	if node == nil {
		return false
	}
	colRef := node.GetColumnRef()
	if colRef == nil {
		return false
	}
	for _, field := range colRef.Fields {
		if str := field.GetString_(); str != nil {
			lower := strings.ToLower(str.Sval)
			if keyColumnMap[lower] {
				return true
			}
		}
	}
	return false
}

// IsSublinkExists checks if a sublink node is an EXISTS subquery.
func IsSublinkExists(subLink *pg_query.SubLink) bool {
	return subLink != nil && subLink.SubLinkType == pg_query.SubLinkType_EXISTS_SUBLINK
}

// IsExemptRegex checks if a raw SQL string qualifies for exemption via regex heuristics.
func IsExemptRegex(sql string) bool {
	if limitRegex.MatchString(sql) {
		return true
	}
	if scalarCountRegex.MatchString(sql) && !groupByRegex.MatchString(sql) {
		return true
	}
	if pointLookupRegex.MatchString(sql) {
		return true
	}
	return false
}
