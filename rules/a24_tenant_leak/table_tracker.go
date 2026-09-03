package a24_tenant_leak

import (
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// TableRef represents a table reference in a SQL query and its active alias.
type TableRef struct {
	Name  string // normalized lowercase table name, e.g. "users"
	Alias string // normalized lowercase alias name, e.g. "u" (or equal to Name)
}

// extractTableRefsFromNodes extracts all table references from AST FROM clause nodes.
func extractTableRefsFromNodes(nodes []*pg_query.Node) []TableRef {
	var refs []TableRef
	for _, n := range nodes {
		refs = append(refs, extractTableRefsFromNode(n)...)
	}
	return refs
}

func extractTableRefsFromNode(node *pg_query.Node) []TableRef {
	if node == nil {
		return nil
	}

	var refs []TableRef
	if rv := node.GetRangeVar(); rv != nil {
		if rv.Relname != "" {
			name := strings.ToLower(rv.Relname)
			alias := name
			if rv.Alias != nil && rv.Alias.Aliasname != "" {
				alias = strings.ToLower(rv.Alias.Aliasname)
			}
			refs = append(refs, TableRef{
				Name:  name,
				Alias: alias,
			})
		}
	}

	if jn := node.GetJoinExpr(); jn != nil {
		refs = append(refs, extractTableRefsFromNode(jn.Larg)...)
		refs = append(refs, extractTableRefsFromNode(jn.Rarg)...)
	}

	if rts := node.GetRangeTableSample(); rts != nil && rts.Relation != nil {
		refs = append(refs, extractTableRefsFromNode(rts.Relation)...)
	}

	return refs
}

// extractJoinQualsFromNodes collects all JOIN ON expression clauses from AST FROM nodes.
func extractJoinQualsFromNodes(nodes []*pg_query.Node) []*pg_query.Node {
	var quals []*pg_query.Node
	for _, n := range nodes {
		quals = append(quals, extractJoinQualsFromNode(n)...)
	}
	return quals
}

func extractJoinQualsFromNode(node *pg_query.Node) []*pg_query.Node {
	if node == nil {
		return nil
	}
	var quals []*pg_query.Node
	if jn := node.GetJoinExpr(); jn != nil {
		if jn.Quals != nil {
			quals = append(quals, jn.Quals)
		}
		quals = append(quals, extractJoinQualsFromNode(jn.Larg)...)
		quals = append(quals, extractJoinQualsFromNode(jn.Rarg)...)
	}
	return quals
}

func getAExprOp(aexpr *pg_query.A_Expr) string {
	if aexpr == nil || len(aexpr.Name) == 0 {
		return ""
	}
	for _, n := range aexpr.Name {
		if s := n.GetString_(); s != nil {
			return s.Sval
		}
	}
	return ""
}

func isIsolatingOp(aexpr *pg_query.A_Expr) bool {
	if aexpr == nil {
		return false
	}
	op := getAExprOp(aexpr)
	switch aexpr.Kind {
	case pg_query.A_Expr_Kind_AEXPR_OP:
		return op == "="
	case pg_query.A_Expr_Kind_AEXPR_IN:
		return op == "=" // In pg_query_go, IN has name "="; NOT IN has name "<>"
	case pg_query.A_Expr_Kind_AEXPR_OP_ANY:
		return op == "=" // e.g. = ANY(...)
	default:
		return false
	}
}

func columnBindsTableTenant(node *pg_query.Node, t TableRef, totalTenantTables int, targetCols map[string]bool) bool {
	if node == nil {
		return false
	}
	col := node.GetColumnRef()
	if col == nil || len(col.Fields) == 0 {
		return false
	}

	lastField := col.Fields[len(col.Fields)-1]
	colStr := lastField.GetString_()
	if colStr == nil || !targetCols[strings.ToLower(colStr.Sval)] {
		return false
	}

	// If exactly 1 tenant table is in the query, any tenant column binds it
	if totalTenantTables == 1 {
		return true
	}

	// If multiple tenant tables are in query, require explicit table/alias match
	if len(col.Fields) < 2 {
		return false
	}

	qualField := col.Fields[len(col.Fields)-2]
	qualStr := qualField.GetString_()
	if qualStr == nil {
		return false
	}

	qual := strings.ToLower(qualStr.Sval)
	return qual == t.Alias || qual == t.Name
}

