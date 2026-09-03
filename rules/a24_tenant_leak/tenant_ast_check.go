// Package a24_tenant_leak validates SQL AST for tenant isolation predicates.
package a24_tenant_leak

import (
	"fmt"
	"regexp"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// CheckTenantQuery inspects an SQL query to ensure multi-tenant tables have a tenant predicate.
func CheckTenantQuery(sql string, tc *TenantConfig) (isViolating bool, reason string) {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" || tc == nil {
		return false, ""
	}

	res, err := pg_query.Parse(trimmed)
	if err == nil && res != nil {
		for _, stmt := range res.Stmts {
			if stmt == nil || stmt.Stmt == nil {
				continue
			}
			if violating, msg := checkStmtNode(stmt.Stmt.Node, tc); violating {
				return true, msg
			}
		}
		return false, ""
	}

	// Fallback regex analysis if pg_query parsing fails
	return checkRegexFallback(trimmed, tc)
}

func checkStmtNode(node any, tc *TenantConfig) (bool, string) {
	if node == nil {
		return false, ""
	}

	switch n := node.(type) {
	case *pg_query.Node_SelectStmt:
		return checkSelect(n.SelectStmt, tc)
	case *pg_query.Node_UpdateStmt:
		return checkUpdate(n.UpdateStmt, tc)
	case *pg_query.Node_DeleteStmt:
		return checkDelete(n.DeleteStmt, tc)
	}

	return false, ""
}

func checkSelect(sel *pg_query.SelectStmt, tc *TenantConfig) (bool, string) {
	if sel == nil {
		return false, ""
	}

	tables := extractTablesFromNodes(sel.FromClause)
	for _, table := range tables {
		if tc.IsTenantTable(table) {
			if !hasTenantColumnInNode(sel.WhereClause, tc) {
				return true, fmt.Sprintf("query on multi-tenant table '%s' missing '%s' predicate; risk of cross-tenant data breach (CWE-284, OWASP API1:2023 BOLA)", table, tc.TenantColumn)
			}
		}
	}

	// Inspect CTEs
	if sel.WithClause != nil {
		for _, cteNode := range sel.WithClause.Ctes {
			if cte := cteNode.GetCommonTableExpr(); cte != nil && cte.Ctequery != nil {
				if violating, msg := checkStmtNode(cte.Ctequery.Node, tc); violating {
					return true, msg
				}
			}
		}
	}

	return false, ""
}

func checkUpdate(upd *pg_query.UpdateStmt, tc *TenantConfig) (bool, string) {
	if upd == nil || upd.Relation == nil {
		return false, ""
	}

	table := upd.Relation.Relname
	if tc.IsTenantTable(table) {
		if !hasTenantColumnInNode(upd.WhereClause, tc) {
			return true, fmt.Sprintf("UPDATE on multi-tenant table '%s' missing '%s' predicate; risk of cross-tenant data mutation (CWE-284, OWASP API1:2023 BOLA)", table, tc.TenantColumn)
		}
	}

	return false, ""
}

func checkDelete(del *pg_query.DeleteStmt, tc *TenantConfig) (bool, string) {
	if del == nil || del.Relation == nil {
		return false, ""
	}

	table := del.Relation.Relname
	if tc.IsTenantTable(table) {
		if !hasTenantColumnInNode(del.WhereClause, tc) {
			return true, fmt.Sprintf("DELETE on multi-tenant table '%s' missing '%s' predicate; risk of cross-tenant data deletion (CWE-284, OWASP API1:2023 BOLA)", table, tc.TenantColumn)
		}
	}

	return false, ""
}

func extractTablesFromNodes(nodes []*pg_query.Node) []string {
	var tables []string
	for _, n := range nodes {
		tables = append(tables, extractTablesFromNode(n)...)
	}
	return tables
}

func extractTablesFromNode(node *pg_query.Node) []string {
	if node == nil {
		return nil
	}

	var tables []string
	if rv := node.GetRangeVar(); rv != nil {
		if rv.Relname != "" {
			tables = append(tables, rv.Relname)
		}
	}

	if jn := node.GetJoinExpr(); jn != nil {
		tables = append(tables, extractTablesFromNode(jn.Larg)...)
		tables = append(tables, extractTablesFromNode(jn.Rarg)...)
	}

	if rts := node.GetRangeTableSample(); rts != nil && rts.Relation != nil {
		tables = append(tables, extractTablesFromNode(rts.Relation)...)
	}

	return tables
}

func hasTenantColumnInNode(node *pg_query.Node, tc *TenantConfig) bool {
	if node == nil || tc == nil {
		return false
	}

	targetCols := map[string]bool{
		strings.ToLower(tc.TenantColumn): true,
		"tenant_id":                      true,
		"org_id":                         true,
		"organization_id":                true,
	}

	return inspectConjunctiveTenantNode(node, targetCols)
}

func inspectConjunctiveTenantNode(node *pg_query.Node, targetCols map[string]bool) bool {
	if node == nil {
		return false
	}

	// 1. BoolExpr: AND requires ANY branch, OR requires ALL branches, NOT is inverted (unsafe)
	if bexpr := node.GetBoolExpr(); bexpr != nil {
		switch bexpr.Boolop {
		case pg_query.BoolExprType_AND_EXPR:
			for _, arg := range bexpr.Args {
				if inspectConjunctiveTenantNode(arg, targetCols) {
					return true
				}
			}
			return false

		case pg_query.BoolExprType_OR_EXPR:
			if len(bexpr.Args) == 0 {
				return false
			}
			for _, arg := range bexpr.Args {
				if !inspectConjunctiveTenantNode(arg, targetCols) {
					return false
				}
			}
			return true

		case pg_query.BoolExprType_NOT_EXPR:
			return false
		}
	}

	// 2. Direct binary comparison expression: e.g. tenant_id = $1, tenant_id IN (...)
	if aexpr := node.GetAExpr(); aexpr != nil {
		return containsTenantColumn(aexpr.Lexpr, targetCols) || containsTenantColumn(aexpr.Rexpr, targetCols)
	}

	// 3. SubLink comparison: e.g. tenant_id IN (SELECT ...)
	if sub := node.GetSubLink(); sub != nil {
		return containsTenantColumn(sub.Testexpr, targetCols)
	}

	// 4. Null test: e.g. tenant_id IS NOT NULL
	if nullTest := node.GetNullTest(); nullTest != nil {
		return containsTenantColumn(nullTest.Arg, targetCols)
	}

	return false
}

func containsTenantColumn(node *pg_query.Node, targetCols map[string]bool) bool {
	if node == nil {
		return false
	}
	if col := node.GetColumnRef(); col != nil {
		for _, field := range col.Fields {
			if s := field.GetString_(); s != nil {
				if targetCols[strings.ToLower(s.Sval)] {
					return true
				}
			}
		}
	}
	return false
}

func checkRegexFallback(sql string, tc *TenantConfig) (bool, string) {
	col := tc.TenantColumn
	if col == "" {
		col = "tenant_id"
	}

	tenantPredRegex := regexp.MustCompile(`(?i)\b(?:[a-zA-Z0-9_]+\.)?(?:` + regexp.QuoteMeta(col) + `|tenant_id|org_id)\s*(?:=|\bIN\b|\bIS\b)`)
	for table := range tc.TenantTables {
		tblRegex := regexp.MustCompile(`(?i)\b(?:FROM|UPDATE|INTO|JOIN)\s+(?:"?` + regexp.QuoteMeta(table) + `"?)`)
		if tblRegex.MatchString(sql) {
			if !tenantPredRegex.MatchString(sql) {
				return true, fmt.Sprintf("query on multi-tenant table '%s' missing '%s' predicate (CWE-284, OWASP API1:2023 BOLA)", table, col)
			}
		}
	}

	return false, ""
}
