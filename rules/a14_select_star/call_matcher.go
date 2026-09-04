// Package a14_select_star identifies database callsites for SELECT * auditing.
package a14_select_star

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

// isDatabaseCall verifies that a call expression targets a genuine database querier.
func isDatabaseCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil || !callsite.IsDBQueryMethod(sel.Sel.Name) {
		return false
	}

	// 1. Semantic Type Resolution via types.Info
	if pass != nil && pass.TypesInfo != nil {
		if recvType := pass.TypesInfo.TypeOf(sel.X); recvType != nil {
			if callsite.IsPgxOrSQLType(recvType) {
				return true
			}
			tName := strings.ToLower(recvType.String())
			return strings.Contains(tName, "db") || strings.Contains(tName, "pool") ||
				strings.Contains(tName, "tx") || strings.Contains(tName, "querier") ||
				strings.Contains(tName, "repo") || strings.Contains(tName, "store")
		}
	}

	// 2. Syntactic heuristic in standalone mode (pass == nil)
	isDBIdent := func(name string) bool {
		lower := strings.ToLower(name)
		return lower == "q" || strings.Contains(lower, "db") || strings.Contains(lower, "pool") ||
			strings.Contains(lower, "tx") || strings.Contains(lower, "conn") ||
			strings.Contains(lower, "repo") || strings.Contains(lower, "store") ||
			strings.Contains(lower, "dao") || strings.Contains(lower, "queries")
	}

	if id, ok := sel.X.(*ast.Ident); ok {
		return isDBIdent(id.Name)
	}
	if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
		return isDBIdent(innerSel.Sel.Name)
	}
	return false
}
