package a17_nplusone

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

func resolveCallObj(info *types.Info, call *ast.CallExpr) types.Object {
	if info == nil || call == nil {
		return nil
	}
	fun := call.Fun
	for {
		if paren, ok := fun.(*ast.ParenExpr); ok {
			fun = paren.X
		} else {
			break
		}
	}

	switch e := fun.(type) {
	case *ast.Ident:
		if obj, ok := info.Uses[e]; ok && obj != nil {
			return obj
		}
		if obj, ok := info.Defs[e]; ok && obj != nil {
			return obj
		}
	case *ast.SelectorExpr:
		if sel, ok := info.Selections[e]; ok && sel != nil {
			return sel.Obj()
		}
		if obj, ok := info.Uses[e.Sel]; ok && obj != nil {
			return obj
		}
	}
	return nil
}

func resolveCallStrKey(pass *analysis.Pass, call *ast.CallExpr) string {
	if call == nil {
		return ""
	}
	fun := call.Fun
	for {
		if paren, ok := fun.(*ast.ParenExpr); ok {
			fun = paren.X
		} else {
			break
		}
	}

	switch e := fun.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		// Attempt receiver type resolution using types.Info
		if pass != nil && pass.TypesInfo != nil {
			if tv, ok := pass.TypesInfo.Types[e.X]; ok && tv.Type != nil {
				recvTypeName := extractTypeNameFromType(tv.Type)
				if recvTypeName != "" {
					return "(" + recvTypeName + ")." + e.Sel.Name
				}
			}
		}
		// Attempt receiver type resolution using AST Ident declaration
		if id, ok := e.X.(*ast.Ident); ok && id.Obj != nil {
			switch decl := id.Obj.Decl.(type) {
			case *ast.Field:
				typeName := extractReceiverTypeName(decl.Type)
				if typeName != "" {
					return "(" + typeName + ")." + e.Sel.Name
				}
			case *ast.ValueSpec:
				typeName := extractReceiverTypeName(decl.Type)
				if typeName != "" {
					return "(" + typeName + ")." + e.Sel.Name
				}
			}
		}
		return e.Sel.Name
	}
	return ""
}

func getFuncDeclKey(fn *ast.FuncDecl) string {
	if fn == nil || fn.Name == nil {
		return ""
	}
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		typeName := extractReceiverTypeName(fn.Recv.List[0].Type)
		if typeName != "" {
			return "(" + typeName + ")." + fn.Name.Name
		}
	}
	return fn.Name.Name
}

func formatObjName(obj types.Object, fn *ast.FuncDecl) string {
	if fn != nil && fn.Recv != nil && len(fn.Recv.List) > 0 {
		typeName := extractReceiverTypeName(fn.Recv.List[0].Type)
		if typeName != "" {
			return fmt.Sprintf("(%s).%s", typeName, fn.Name.Name)
		}
	}
	if fn != nil && fn.Name != nil {
		return fn.Name.Name
	}
	if obj != nil {
		return obj.Name()
	}
	return ""
}

func extractReceiverTypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	for {
		if star, ok := expr.(*ast.StarExpr); ok {
			expr = star.X
		} else if paren, ok := expr.(*ast.ParenExpr); ok {
			expr = paren.X
		} else {
			break
		}
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		return extractReceiverTypeName(e.X)
	case *ast.IndexListExpr:
		return extractReceiverTypeName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

func extractTypeNameFromType(t types.Type) string {
	if t == nil {
		return ""
	}
	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}
