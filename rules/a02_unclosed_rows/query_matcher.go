// Package a02_unclosed_rows enforces that every pgx.Rows produced by Query()
// is safely closed via defer rows.Close() or consumed by an auto-closing helper.
package a02_unclosed_rows

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

// IsQueryCall determines whether an AST expression is a database Query call producing a rows cursor.
func IsQueryCall(pass *analysis.Pass, e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil || sel.Sel.Name != "Query" {
		return false
	}

	if id, ok := sel.X.(*ast.Ident); ok {
		if isNonDBReceiverName(id.Name) {
			return false
		}
	}

	// 1. Type-aware discrimination when TypesInfo is available
	if pass != nil && pass.TypesInfo != nil {
		return isLegitDatabaseQuery(pass, call, sel)
	}

	// 2. AST-level heuristic fallback (e.g. standalone CLI mode without types)
	return isDatabaseQueryAST(call, sel)
}

func isLegitDatabaseQuery(pass *analysis.Pass, call *ast.CallExpr, sel *ast.SelectorExpr) bool {
	// Must return a cursor that has a Close() method or is named Rows
	if !returnsCloseableCursor(pass, call) {
		return false
	}

	var recvType types.Type
	if selType, ok := pass.TypesInfo.Selections[sel]; ok && selType.Recv() != nil {
		recvType = selType.Recv()
	} else if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.Type != nil {
		recvType = tv.Type
	}

	if recvType != nil {
		if callsite.IsPgxOrSQLType(recvType) {
			return true
		}
		typeStr := strings.ToLower(recvType.String())
		if isNonDBTypeString(typeStr) {
			return false
		}
		return true
	}

	return isDatabaseQueryAST(call, sel)
}

func returnsCloseableCursor(pass *analysis.Pass, call *ast.CallExpr) bool {
	tv, ok := pass.TypesInfo.Types[call]
	if !ok || tv.Type == nil {
		return true // fallback when type is unrecorded
	}

	t := tv.Type
	if tuple, ok := t.(*types.Tuple); ok && tuple.Len() > 0 {
		cursorType := tuple.At(0).Type()
		return typeHasCloseMethod(cursorType)
	}
	return typeHasCloseMethod(t)
}

func typeHasCloseMethod(t types.Type) bool {
	if t == nil {
		return false
	}
	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}
	if named, ok := t.(*types.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			if named.Method(i).Name() == "Close" {
				return true
			}
		}
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			if iface.Method(i).Name() == "Close" {
				return true
			}
		}
	}
	s := strings.ToLower(t.String())
	return strings.Contains(s, "rows") || strings.Contains(s, "cursor")
}

func isNonDBReceiverName(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "search", "searchengine", "client", "http", "httpclient", "logger", "log",
		"queue", "cmd", "url", "req", "response", "engine", "es", "solr", "graphql":
		return true
	}
	return false
}

func isNonDBTypeString(s string) bool {
	switch {
	case strings.Contains(s, "search"), strings.Contains(s, "http"),
		strings.Contains(s, "logger"), strings.Contains(s, "queue"),
		strings.Contains(s, "graphql"), strings.Contains(s, "solr"):
		return true
	}
	return false
}

func isDatabaseQueryAST(call *ast.CallExpr, sel *ast.SelectorExpr) bool {
	if call == nil || len(call.Args) == 0 {
		return false
	}
	if innerSel, ok := sel.X.(*ast.SelectorExpr); ok && innerSel.Sel.Name == "URL" {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); ok {
		if isNonDBReceiverName(id.Name) {
			return false
		}
		if isKnownDBReceiverName(id.Name) {
			return true
		}
	}
	if selX, ok := sel.X.(*ast.SelectorExpr); ok {
		if isKnownDBReceiverName(selX.Sel.Name) {
			return true
		}
	}
	return true
}

func isKnownDBReceiverName(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "db", "pool", "tx", "conn", "queries", "querier", "database", "store", "repo", "repository", "r", "exec":
		return true
	}
	for _, suffix := range []string{"db", "pool", "tx", "conn", "repo", "store"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
