// Package a12_timeout_config verifies pgxpool.Config type identities.
package a12_timeout_config

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// isPgxpoolConfigArg verifies that cfgArg passed to pgxpool.NewWithConfig
// is proven to be an actual *pgxpool.Config (or pgxpool.Config) object.
func isPgxpoolConfigArg(pass *analysis.Pass, file *ast.File, expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		named, pkg := resolveNamedConfig(pass, expr)
		if named == nil || pkg == nil || named.Obj().Name() != "Config" {
			return false
		}
		if isPgxpoolPath(pkg.Path()) {
			return true
		}
		if st, ok := named.Underlying().(*types.Struct); ok {
			return isPgxpoolConfigStructType(st)
		}
		return false
	}
	return isPgxpoolConfigArgAST(file, expr)
}

func resolveNamedConfig(pass *analysis.Pass, expr ast.Expr) (*types.Named, *types.Package) {
	if pass == nil || pass.TypesInfo == nil || expr == nil {
		return nil, nil
	}
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		if id, ok := expr.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				t = obj.Type()
			} else if obj := pass.TypesInfo.Defs[id]; obj != nil {
				t = obj.Type()
			}
		}
	}
	for ptr, ok := t.(*types.Pointer); ok; ptr, ok = t.(*types.Pointer) {
		t = ptr.Elem()
	}
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		return named, named.Obj().Pkg()
	}
	return nil, nil
}

func isPgxpoolConfigArgAST(file *ast.File, expr ast.Expr) bool {
	if file == nil || expr == nil {
		return false
	}
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}
	if lit, ok := expr.(*ast.CompositeLit); ok {
		return isPgxpoolConfigType(nil, file, lit.Type)
	}
	id, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	fn := findEnclosingFunc(file, id.Pos())
	if fn == nil || fn.Body == nil {
		return false
	}
	return traceIdentConfigTypeAST(file, fn, id)
}

func traceIdentConfigTypeAST(file *ast.File, fn *ast.FuncDecl, targetID *ast.Ident) bool {
	if fn.Type != nil && fn.Type.Params != nil {
		for _, param := range fn.Type.Params.List {
			for _, name := range param.Names {
				if name.Name == targetID.Name {
					return isPgxpoolConfigTypeExpr(file, param.Type)
				}
			}
		}
	}

	targetDeclPos := findDeclPos(fn.Body, targetID)
	var foundType ast.Expr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if n == nil || n.Pos() >= targetID.Pos() {
			return false
		}
		switch s := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					matches := (targetDeclPos != token.NoPos && id.Pos() == targetDeclPos) ||
						(targetDeclPos == token.NoPos && id.Name == targetID.Name)
					if matches {
						var rhs ast.Expr
						if i < len(s.Rhs) {
							rhs = s.Rhs[i]
						} else if len(s.Rhs) == 1 {
							rhs = s.Rhs[0]
						}
						if rhs != nil {
							foundType = extractTypeFromRHS(file, rhs)
						}
					}
				}
			}
		case *ast.DeclStmt:
			if gen, ok := s.Decl.(*ast.GenDecl); ok {
				for _, spec := range gen.Specs {
					if valSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valSpec.Names {
							matches := (targetDeclPos != token.NoPos && name.Pos() == targetDeclPos) ||
								(targetDeclPos == token.NoPos && name.Name == targetID.Name)
							if matches && valSpec.Type != nil {
								foundType = valSpec.Type
							}
						}
					}
				}
			}
		}
		return true
	})

	return foundType != nil && isPgxpoolConfigTypeExpr(file, foundType)
}

func extractTypeFromRHS(file *ast.File, rhs ast.Expr) ast.Expr {
	if unary, ok := rhs.(*ast.UnaryExpr); ok {
		rhs = unary.X
	}
	if lit, ok := rhs.(*ast.CompositeLit); ok {
		return lit.Type
	}
	if call, ok := rhs.(*ast.CallExpr); ok {
		fnName := exprToString(call.Fun)
		if strings.HasSuffix(fnName, "ParseConfig") {
			return &ast.SelectorExpr{X: &ast.Ident{Name: "pgxpool"}, Sel: &ast.Ident{Name: "Config"}}
		}
		targetFn := findFuncDeclByName(file, fnName)
		if targetFn != nil && targetFn.Type != nil && targetFn.Type.Results != nil && len(targetFn.Type.Results.List) > 0 {
			return targetFn.Type.Results.List[0].Type
		}
	}
	return nil
}

func isPgxpoolConfigTypeExpr(file *ast.File, typeExpr ast.Expr) bool {
	if typeExpr == nil {
		return false
	}
	for star, ok := typeExpr.(*ast.StarExpr); ok; star, ok = typeExpr.(*ast.StarExpr) {
		typeExpr = star.X
	}
	if sel, ok := typeExpr.(*ast.SelectorExpr); ok && sel.Sel.Name == "Config" {
		if id, ok := sel.X.(*ast.Ident); ok {
			if id.Name == "pgxpool" {
				return true
			}
			if target, ok := findPgxpoolImport(file); ok && id.Name == target {
				return true
			}
		}
	}
	if id, ok := typeExpr.(*ast.Ident); ok && id.Name == "Config" {
		return isConfigStructDeclaredWithConnConfig(file, "Config")
	}
	return false
}

func isPgxpoolConfigType(pass *analysis.Pass, file *ast.File, expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		named, pkg := resolveNamedConfig(pass, expr)
		if named == nil || pkg == nil || named.Obj().Name() != "Config" {
			return false
		}
		if isPgxpoolPath(pkg.Path()) {
			return true
		}
		if st, ok := named.Underlying().(*types.Struct); ok {
			return isPgxpoolConfigStructType(st)
		}
		return false
	}
	return isPgxpoolConfigTypeExpr(file, expr)
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
		hasConnConfig = hasConnConfig || (name == "ConnConfig")
		hasPoolParam = hasPoolParam || isPoolParam(name)
	}
	return hasConnConfig && hasPoolParam
}

func isConfigStructDeclaredWithConnConfig(file *ast.File, typeName string) bool {
	if file == nil || typeName != "Config" {
		return false
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != "Config" {
				continue
			}
			st, ok := typeSpec.Type.(*ast.StructType)
			if !ok || st.Fields == nil {
				continue
			}
			var hasConnConfig, hasPoolParam bool
			for _, f := range st.Fields.List {
				for _, name := range f.Names {
					hasConnConfig = hasConnConfig || (name.Name == "ConnConfig")
					hasPoolParam = hasPoolParam || isPoolParam(name.Name)
				}
			}
			return hasConnConfig && hasPoolParam
		}
	}
	return false
}
