// Package a07_error_leak provides AST-level database type and constructor resolution
// when compiler type checker information is absent or partial.
package a07_error_leak

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/dbident"
)

func isKnownDBDriverASTType(expr ast.Expr, file *ast.File, fn *ast.FuncDecl) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkgID, ok := sel.X.(*ast.Ident); ok {
			if isKnownDBPackageIdent(nil, file, fn, pkgID) {
				switch sel.Sel.Name {
				case "DB", "Tx", "Conn", "Pool", "Batch", "BatchResults", "Stmt", "Rows", "Row", "Result", "CommandTag":
					return true
				}
			}
		}
	}
	return false
}

func isProvenDBQuerierASTType(expr ast.Expr, file *ast.File, fn *ast.FuncDecl) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if isKnownDBDriverASTType(expr, file, fn) {
		return true
	}
	if id, ok := expr.(*ast.Ident); ok {
		ts := findTypeSpec(id.Name, file)
		if ts != nil {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok && iface.Methods != nil {
				hasExec, hasQuery := false, false
				for _, m := range iface.Methods.List {
					ft, ok := m.Type.(*ast.FuncType)
					if !ok {
						continue
					}
					for _, nm := range m.Names {
						switch nm.Name {
						case "Exec", "ExecContext":
							if isASTExecOrQueryMethod(ft, file, fn) {
								hasExec = true
							}
						case "Query", "QueryContext":
							if isASTExecOrQueryMethod(ft, file, fn) {
								hasQuery = true
							}
						}
					}
				}
				return hasExec || hasQuery
			}
		}
	}
	return false
}

func isASTExecOrQueryMethod(ft *ast.FuncType, file *ast.File, fn *ast.FuncDecl) bool {
	if ft == nil || ft.Results == nil || len(ft.Results.List) != 2 {
		return false
	}
	if !isKnownDBDriverASTType(ft.Results.List[0].Type, file, fn) {
		return false
	}
	if errID, ok := ft.Results.List[1].Type.(*ast.Ident); !ok || errID.Name != "error" {
		return false
	}
	return true
}

func isAssignedFromDBConstructor(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, expr ast.Expr) bool {
	id, ok := expr.(*ast.Ident)
	if !ok || fn == nil || fn.Body == nil {
		return false
	}
	var isConstructor bool
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if isConstructor {
			return false
		}
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name && i < len(as.Rhs) {
				if call, ok := as.Rhs[i].(*ast.CallExpr); ok && isDBConstructorCall(pass, file, fn, call) {
					isConstructor = true
					return false
				}
			}
		}
		return true
	})
	return isConstructor
}

func isDBConstructorCall(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if !isKnownDBPackageIdent(pass, file, fn, id) {
		return false
	}
	return dbident.IsDBConstructorMethod(sel.Sel.Name)
}
