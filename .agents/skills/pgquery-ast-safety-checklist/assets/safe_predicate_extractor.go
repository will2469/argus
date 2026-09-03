package astsafety

import (
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// HasConjunctiveColumnPredicate verifies that a target column is guaranteed to be evaluated
// as a mandatory conjunctive filter (under top-level AND_EXPR, never inside an OR_EXPR or NOT_EXPR).
// This prevents the critical A24 security bypass where 'OR tenant_id = $1' falsely passed isolation checks.
func HasConjunctiveColumnPredicate(whereClause *pg_query.Node, targetColNames ...string) bool {
	if whereClause == nil || len(targetColNames) == 0 {
		return false
	}

	targets := make(map[string]bool)
	for _, col := range targetColNames {
		targets[strings.ToLower(strings.TrimSpace(col))] = true
	}

	return inspectConjunctiveNode(whereClause, targets)
}

func inspectConjunctiveNode(node *pg_query.Node, targets map[string]bool) bool {
	if node == nil {
		return false
	}

	// 1. If it's a BoolExpr, ONLY traverse AND_EXPR conjuncts
	if bexpr := node.GetBoolExpr(); bexpr != nil {
		switch bexpr.Boolop {
		case pg_query.BoolExprType_AND_EXPR:
			// Under an AND, if ANY child provides a safe conjunctive check for target column,
			// the entire query is guaranteed to filter by that target column!
			for _, arg := range bexpr.Args {
				if inspectConjunctiveNode(arg, targets) {
					return true
				}
			}
			return false

		case pg_query.BoolExprType_OR_EXPR:
			// CRITICAL GOTCHA (A24 TRAP):
			// In (A OR tenant_id = $1), if A is true, the row is selected WITHOUT matching tenant_id.
			// A predicate inside an OR is NEVER a universal conjunctive invariant!
			return false

		case pg_query.BoolExprType_NOT_EXPR:
			// In NOT (tenant_id = $1), the condition is negated!
			return false
		}
	}

	// 2. Direct binary comparison expression: e.g. tenant_id = $1, tenant_id IN (...)
	if aexpr := node.GetAExpr(); aexpr != nil {
		if !isIsolatingOperator(aexpr) {
			return false
		}
		if hasTargetColumn(aexpr.Lexpr, targets) || hasTargetColumn(aexpr.Rexpr, targets) {
			return true
		}
	}

	// 3. SubLink comparison: e.g. tenant_id IN (SELECT ...)
	if subLink := node.GetSubLink(); subLink != nil {
		if subLink.SubLinkType == pg_query.SubLinkType_ANY_SUBLINK && hasTargetColumn(subLink.Testexpr, targets) {
			return true
		}
	}

	// 4. NullTest (e.g. tenant_id IS NOT NULL) is strictly non-isolating (never true)
	return false
}

func isIsolatingOperator(aexpr *pg_query.A_Expr) bool {
	if aexpr == nil || len(aexpr.Name) == 0 {
		return false
	}
	op := ""
	for _, n := range aexpr.Name {
		if s := n.GetString_(); s != nil {
			op = s.Sval
		}
	}
	switch aexpr.Kind {
	case pg_query.A_Expr_Kind_AEXPR_OP:
		return op == "="
	case pg_query.A_Expr_Kind_AEXPR_IN:
		return op == "="
	case pg_query.A_Expr_Kind_AEXPR_OP_ANY:
		return op == "="
	default:
		return false
	}
}

func hasTargetColumn(node *pg_query.Node, targets map[string]bool) bool {
	if node == nil {
		return false
	}
	if col := node.GetColumnRef(); col != nil {
		for _, field := range col.Fields {
			if s := field.GetString_(); s != nil {
				if targets[strings.ToLower(s.Sval)] {
					return true
				}
			}
		}
	}
	return false
}
