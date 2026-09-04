// Package dbident provides common type utilities shared across all Argus
// rules, eliminating duplicated copies of unwrapPointer, hasInvalidType,
// getTypeName, getASTTypeName, and findTypeSpec.
package dbident

import (
	"go/ast"
	"go/token"
	"go/types"
)

// UnwrapPointer recursively unwraps pointer types to get the element type.
func UnwrapPointer(t types.Type) types.Type {
	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			return t
		}
	}
}

// HasInvalidType reports whether t contains an Invalid basic type,
// indicating type-checker failure for the node or its sub-components.
// Uses a visited set to safely handle recursive types and avoid infinite loops.
func HasInvalidType(t types.Type) bool {
	return hasInvalidTypeVisited(t, make(map[types.Type]bool))
}

func hasInvalidTypeVisited(t types.Type, visited map[types.Type]bool) bool {
	if t == nil {
		return true
	}
	if visited[t] {
		return false
	}
	visited[t] = true

	switch x := t.(type) {
	case *types.Basic:
		return x.Kind() == types.Invalid
	case *types.Pointer:
		return hasInvalidTypeVisited(x.Elem(), visited)
	case *types.Named:
		return hasInvalidTypeVisited(x.Underlying(), visited)
	case *types.Interface:
		for i := 0; i < x.NumMethods(); i++ {
			if hasInvalidTypeVisited(x.Method(i).Type(), visited) {
				return true
			}
		}
		for i := 0; i < x.NumEmbeddeds(); i++ {
			if hasInvalidTypeVisited(x.EmbeddedType(i), visited) {
				return true
			}
		}
	case *types.Signature:
		if params := x.Params(); params != nil {
			for i := 0; i < params.Len(); i++ {
				if hasInvalidTypeVisited(params.At(i).Type(), visited) {
					return true
				}
			}
		}
		if results := x.Results(); results != nil {
			for i := 0; i < results.Len(); i++ {
				if hasInvalidTypeVisited(results.At(i).Type(), visited) {
					return true
				}
			}
		}
	case *types.Tuple:
		for i := 0; i < x.Len(); i++ {
			if hasInvalidTypeVisited(x.At(i).Type(), visited) {
				return true
			}
		}
	case *types.Slice:
		return hasInvalidTypeVisited(x.Elem(), visited)
	case *types.Array:
		return hasInvalidTypeVisited(x.Elem(), visited)
	case *types.Map:
		return hasInvalidTypeVisited(x.Key(), visited) || hasInvalidTypeVisited(x.Elem(), visited)
	case *types.Chan:
		return hasInvalidTypeVisited(x.Elem(), visited)
	}
	return false
}

// GetTypeName extracts the declared type name from a types.Type,
// unwrapping pointers first.
func GetTypeName(t types.Type) string {
	if t == nil {
		return ""
	}
	t = UnwrapPointer(t)
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		return named.Obj().Name()
	}
	return ""
}

// GetASTTypeName extracts the type name from an AST type expression,
// unwrapping star, index, and selector expressions.
func GetASTTypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		return GetASTTypeName(e.X)
	case *ast.IndexListExpr:
		return GetASTTypeName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

// FindTypeSpec looks up a type declaration by name in an AST file.
func FindTypeSpec(name string, file *ast.File) *ast.TypeSpec {
	if file == nil || name == "" {
		return nil
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == name {
				return ts
			}
		}
	}
	return nil
}
