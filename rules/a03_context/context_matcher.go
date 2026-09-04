// Package a03_context detects database operations executed with unbounded raw contexts
// such as context.Background() or context.TODO(), enforcing request deadlines or timeouts.
package a03_context

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// isRawContextCall checks whether a call expression is context.Background() or context.TODO(),
// taking into account package import aliases (e.g. import stdctx "context").
func isRawContextCall(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	isCtx, fnName := isContextPackageSelector(pass, file, sel)
	if !isCtx {
		return false
	}
	return fnName == "Background" || fnName == "TODO"
}

// isBoundedContextCall checks whether a call produces a bounded or cancellable context.
func isBoundedContextCall(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// 1. Method named Context (e.g. r.Context(), c.Request.Context())
	if sel.Sel.Name == "Context" {
		return true
	}

	// 2. Standard library context functions: WithTimeout, WithDeadline, WithCancel, WithCancelCause
	isCtx, fnName := isContextPackageSelector(pass, file, sel)
	if isCtx {
		switch fnName {
		case "WithTimeout", "WithDeadline", "WithCancel", "WithCancelCause":
			return true
		}
	}

	// 3. signal.NotifyContext from os/signal
	if isSignalNotifyContext(pass, file, sel) {
		return true
	}

	return false
}

func isContextPackageSelector(pass *analysis.Pass, file *ast.File, sel *ast.SelectorExpr) (bool, string) {
	if sel == nil {
		return false, ""
	}

	// 1. Type-aware resolution when TypesInfo is present
	if pass != nil && pass.TypesInfo != nil {
		if id, ok := sel.X.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				if pkgName, ok := obj.(*types.PkgName); ok && pkgName.Imported().Path() == "context" {
					return true, sel.Sel.Name
				}
			}
		}
	}

	// 2. AST-level import map resolution (handles aliased imports in standalone mode)
	if file != nil {
		aliases := getContextImportAliases(file)
		if id, ok := sel.X.(*ast.Ident); ok {
			if aliases[id.Name] {
				return true, sel.Sel.Name
			}
		}
	}

	// 3. Last resort fallback
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "context" {
		return true, sel.Sel.Name
	}

	return false, ""
}

func getContextImportAliases(file *ast.File) map[string]bool {
	aliases := make(map[string]bool)
	for _, imp := range file.Imports {
		pathVal := strings.Trim(imp.Path.Value, "`\"")
		if pathVal == "context" {
			if imp.Name != nil {
				aliases[imp.Name.Name] = true
			} else {
				aliases["context"] = true
			}
		}
	}
	if len(aliases) == 0 {
		aliases["context"] = true
	}
	return aliases
}

func isSignalNotifyContext(pass *analysis.Pass, file *ast.File, sel *ast.SelectorExpr) bool {
	if sel.Sel.Name != "NotifyContext" {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		if id, ok := sel.X.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				if pkgName, ok := obj.(*types.PkgName); ok && pkgName.Imported().Path() == "os/signal" {
					return true
				}
			}
		}
	}
	if file != nil {
		for _, imp := range file.Imports {
			pathVal := strings.Trim(imp.Path.Value, "`\"")
			if pathVal == "os/signal" {
				pkgAlias := "signal"
				if imp.Name != nil {
					pkgAlias = imp.Name.Name
				}
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == pkgAlias {
					return true
				}
			}
		}
	}
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "signal" {
		return true
	}
	return false
}
