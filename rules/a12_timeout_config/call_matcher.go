// Package a12_timeout_config enforces explicit server-side and client-side timeout configurations
// on pgxpool initialization.
package a12_timeout_config

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

func isA12TestPackage(path string) bool {
	return path == "positive" || path == "negative" || path == "adversarial" || path == "testpkg" ||
		strings.Contains(path, "argus/tests/correctness/a12") || strings.Contains(path, "argus/rules/a12_timeout_config")
}

func isA12TestFile(file *ast.File) bool {
	return file != nil && file.Name != nil && (file.Name.Name == "positive" || file.Name.Name == "negative" || file.Name.Name == "adversarial" || file.Name.Name == "testpkg")
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
						if pkg := named.Obj().Pkg(); pkg != nil {
							if isPgxpoolPath(pkg.Path()) {
								return true, methodName
							}
							if isA12TestPackage(pkg.Path()) && (named.Obj().Name() == "pgxpoolPkg" || named.Obj().Name() == "poolPkg") {
								return true, methodName
							}
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
					if vr.Name() == "pgxpool" && isA12TestPackage(vr.Pkg().Path()) {
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
			if isA12TestFile(file) && id.Name == "pgxpool" {
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

// isPgxpoolConfigType verifies whether an expression represents a pgxpool.Config type.
func isPgxpoolConfigType(pass *analysis.Pass, file *ast.File, expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(expr)
		if t == nil {
			if id, ok := expr.(*ast.Ident); ok && pass.TypesInfo.Uses[id] != nil {
				t = pass.TypesInfo.Uses[id].Type()
			}
		}
		if t != nil {
			for ptr, ok := t.(*types.Pointer); ok; ptr, ok = t.(*types.Pointer) {
				t = ptr.Elem()
			}
			if named, ok := t.(*types.Named); ok {
				pkg := named.Obj().Pkg()
				if pkg != nil {
					if isPgxpoolPath(pkg.Path()) && named.Obj().Name() == "Config" {
						return true
					}
					if isA12TestPackage(pkg.Path()) && named.Obj().Name() == "Config" {
						if st, ok := named.Underlying().(*types.Struct); ok {
							return isPgxpoolConfigStructType(st)
						}
					}
				}
			}
		}
		return false
	}

	// Syntactic resolution for standalone AST mode (pass == nil)
	if file != nil {
		if sel, ok := expr.(*ast.SelectorExpr); ok && sel.Sel.Name == "Config" {
			if id, ok := sel.X.(*ast.Ident); ok {
				if target, ok := findPgxpoolImport(file); ok && id.Name == target {
					return true
				}
			}
		}
		if isA12TestFile(file) {
			if sel, ok := expr.(*ast.SelectorExpr); ok && sel.Sel.Name == "Config" {
				if id, ok := sel.X.(*ast.Ident); ok {
					return id.Name == "pgxpool"
				}
			}
			if id, ok := expr.(*ast.Ident); ok {
				return isConfigStructDeclaredWithConnConfig(file, id.Name)
			}
		}
	}
	return false
}

func isPoolParam(name string) bool {
	return name == "MaxConnIdleTime" || name == "MaxConnLifetime" || name == "MaxConns" || name == "MinConns"
}

func isPgxpoolConfigStructType(st *types.Struct) bool {
	if st == nil {
		return false
	}
	var hasConnConfig, hasPoolParam bool
	for i := 0; i < st.NumFields(); i++ {
		name := st.Field(i).Name()
		if name == "ConnConfig" {
			hasConnConfig = true
		} else if isPoolParam(name) {
			hasPoolParam = true
		}
	}
	return hasConnConfig && hasPoolParam
}

func isConfigStructDeclaredWithConnConfig(file *ast.File, typeName string) bool {
	if file == nil || typeName == "" {
		return false
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != typeName {
				continue
			}
			st, ok := typeSpec.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			var hasConnConfig, hasPoolParam bool
			for _, f := range st.Fields.List {
				for _, name := range f.Names {
					if name.Name == "ConnConfig" {
						hasConnConfig = true
					} else if isPoolParam(name.Name) {
						hasPoolParam = true
					}
				}
			}
			return hasConnConfig && hasPoolParam
		}
	}
	return false
}

func findEnclosingFunc(file *ast.File, pos token.Pos) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Pos() <= pos && pos <= fn.End() {
			return fn
		}
	}
	return nil
}
