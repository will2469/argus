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
	if err != nil || res == nil {
		// Strict AST determinism: never silently trust a weaker regex fallback on multi-tenant tables.
		// If an unparseable query references any multi-tenant table, report an explicit verification failure.
		for table := range tc.TenantTables {
			tblRegex := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(table) + `\b`)
			if tblRegex.MatchString(trimmed) {
				return true, fmt.Sprintf("unable to verify tenant isolation for table '%s': SQL AST parsing failed (%v); static AST verification is required (CWE-284, OWASP API1:2023 BOLA)", table, err)
			}
		}
		return false, ""
	}

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

	targetCols := map[string]bool{
		strings.ToLower(tc.TenantColumn): true,
		"tenant_id":                      true,
		"org_id":                         true,
		"organization_id":                true,
	}

	allTableRefs := extractTableRefsFromNodes(sel.FromClause)
	var tenantTables []TableRef
	hasTenantPredicate := containsTenantColumn(sel.WhereClause, targetCols)
	for _, tr := range allTableRefs {
		if tc.IsTenantTable(tr.Name) || (hasTenantPredicate && len(allTableRefs) == 1) {
			tenantTables = append(tenantTables, tr)
		}
	}

	joinQuals := extractJoinQualsFromNodes(sel.FromClause)

	for _, tr := range tenantTables {
		isolated := nodeEnforcesTableTenant(sel.WhereClause, tr, len(tenantTables), targetCols)
		if !isolated {
			for _, jq := range joinQuals {
				if nodeEnforcesTableTenant(jq, tr, len(tenantTables), targetCols) {
					isolated = true
					break
				}
			}
		}

		if !isolated {
			return true, fmt.Sprintf("query on multi-tenant table '%s' missing '%s' predicate; risk of cross-tenant data breach (CWE-284, OWASP API1:2023 BOLA)", tr.Name, tc.TenantColumn)
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

	table := strings.ToLower(upd.Relation.Relname)
	targetCols := map[string]bool{
		strings.ToLower(tc.TenantColumn): true,
		"tenant_id":                      true,
		"org_id":                         true,
		"organization_id":                true,
	}
	if tc.IsTenantTable(table) || containsTenantColumn(upd.WhereClause, targetCols) {
		tr := TableRef{Name: table, Alias: table}
		if !nodeEnforcesTableTenant(upd.WhereClause, tr, 1, targetCols) {
			return true, fmt.Sprintf("UPDATE on multi-tenant table '%s' missing '%s' predicate; risk of cross-tenant data mutation (CWE-284, OWASP API1:2023 BOLA)", table, tc.TenantColumn)
		}
	}

	return false, ""
}

func checkDelete(del *pg_query.DeleteStmt, tc *TenantConfig) (bool, string) {
	if del == nil || del.Relation == nil {
		return false, ""
	}

	table := strings.ToLower(del.Relation.Relname)
	targetCols := map[string]bool{
		strings.ToLower(tc.TenantColumn): true,
		"tenant_id":                      true,
		"org_id":                         true,
		"organization_id":                true,
	}
	if tc.IsTenantTable(table) || containsTenantColumn(del.WhereClause, targetCols) {
		tr := TableRef{Name: table, Alias: table}
		if !nodeEnforcesTableTenant(del.WhereClause, tr, 1, targetCols) {
			return true, fmt.Sprintf("DELETE on multi-tenant table '%s' missing '%s' predicate; risk of cross-tenant data deletion (CWE-284, OWASP API1:2023 BOLA)", table, tc.TenantColumn)
		}
	}

	return false, ""
}

func nodeEnforcesTableTenant(node *pg_query.Node, t TableRef, totalTenantTables int, targetCols map[string]bool) bool {
	if node == nil {
		return false
	}

	// 1. BoolExpr: AND requires ANY branch, OR requires ALL branches, NOT is inverted (unsafe)
	if bexpr := node.GetBoolExpr(); bexpr != nil {
		switch bexpr.Boolop {
		case pg_query.BoolExprType_AND_EXPR:
			for _, arg := range bexpr.Args {
				if nodeEnforcesTableTenant(arg, t, totalTenantTables, targetCols) {
					return true
				}
			}
			return false

		case pg_query.BoolExprType_OR_EXPR:
			if len(bexpr.Args) == 0 {
				return false
			}
			for _, arg := range bexpr.Args {
				if !nodeEnforcesTableTenant(arg, t, totalTenantTables, targetCols) {
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
		if !isIsolatingOp(aexpr) {
			return false
		}
		return columnBindsTableTenant(aexpr.Lexpr, t, totalTenantTables, targetCols) ||
			columnBindsTableTenant(aexpr.Rexpr, t, totalTenantTables, targetCols)
	}

	// 3. SubLink comparison: e.g. tenant_id IN (SELECT ...)
	if sub := node.GetSubLink(); sub != nil {
		if sub.SubLinkType == pg_query.SubLinkType_ANY_SUBLINK {
			return columnBindsTableTenant(sub.Testexpr, t, totalTenantTables, targetCols)
		}
	}

	// NullTest (e.g. tenant_id IS NOT NULL) is strictly NON-ISOLATING (never true)
	return false
}
