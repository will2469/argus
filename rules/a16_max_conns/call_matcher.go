// Package a16_max_conns enforces explicit, bounded MaxConns configuration on pgxpool.
package a16_max_conns

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

func isPgxpoolPath(path string) bool {
	return path == "github.com/jackc/pgx/v5/pgxpool" || path == "github.com/jackc/pgx/v4/pgxpool"
}

func findPgxpoolImport(file *ast.File) (string, bool) {
	if file == nil {
		return "", false
	}
	for _, imp := range file.Imports {
		if isPgxpoolPath(strings.Trim(imp.Path.Value, "`\"")) {
			if imp.Name != nil {
				return imp.Name.Name, true
			}
			return "pgxpool", true
		}
	}
	return "", false
}

// isPgxpoolCall checks whether a call is invoking pgxpool.New or pgxpool.NewWithConfig.
// Strictly verifies package type or import path, preventing false positives on unrelated pools.
func isPgxpoolCall(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) (bool, string) {
	if call == nil {
		return false, ""
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil || (sel.Sel.Name != "New" && sel.Sel.Name != "NewWithConfig") {
		return false, ""
	}
	methodName := sel.Sel.Name

	// 1. Semantic Type Resolution via types.Info
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.ObjectOf(sel.Sel); obj != nil {
			if pkg := obj.Pkg(); pkg != nil && isPgxpoolPath(pkg.Path()) {
				return true, methodName
			}
			if fn, ok := obj.(*types.Func); ok {
				if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
					t := sig.Recv().Type()
					for ptr, ok := t.(*types.Pointer); ok; ptr, ok = t.(*types.Pointer) {
						t = ptr.Elem()
					}
					if named, ok := t.(*types.Named); ok {
						if pkg := named.Obj().Pkg(); pkg != nil && isPgxpoolPath(pkg.Path()) {
							return true, methodName
						}
						if named.Obj().Name() == "pgxpoolPkg" || named.Obj().Name() == "poolPkg" {
							return true, methodName
						}
					}
					return false, ""
				}
			}
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				if pkgName, ok := obj.(*types.PkgName); ok {
					return isPgxpoolPath(pkgName.Imported().Path()), methodName
				}
				if vr, ok := obj.(*types.Var); ok {
					if vr.Name() == "pgxpool" {
						return true, methodName
					}
					return false, ""
				}
			}
		}
		return false, ""
	}

	// 2. Syntactic import resolution for standalone AST mode (pass == nil)
	if file != nil {
		if id, ok := sel.X.(*ast.Ident); ok {
			if target, ok := findPgxpoolImport(file); ok && id.Name == target {
				return true, methodName
			}
			if id.Name == "pgxpool" {
				return true, methodName
			}
		}
	}
	return false, ""
}

// findCallArg locates the primary non-context argument (DSN string or Config struct)
// using semantic context identification rather than positional assumptions.
func findCallArg(call *ast.CallExpr, pass *analysis.Pass) (ast.Expr, int) {
	if call == nil || len(call.Args) == 0 {
		return nil, -1
	}
	for i, arg := range call.Args {
		if !callsite.IsContextArg(arg, pass) {
			return arg, i
		}
	}
	return nil, -1
}

func findEnclosingFunc(file *ast.File, pos token.Pos) *ast.FuncDecl {
	if file == nil {
		return nil
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Pos() <= pos && pos <= fn.End() {
			return fn
		}
	}
	return nil
}
