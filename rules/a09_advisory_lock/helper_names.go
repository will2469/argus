// Package a09_advisory_lock provides approved naming definitions and AST type helpers.
package a09_advisory_lock

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/will2469/argus/shared/dbident"
)

func isArgusPackagePath(path string) bool {
	if strings.Contains(path, "/tests/") || strings.HasSuffix(path, "/tests") {
		return false
	}
	return path == "github.com/will2469/argus" ||
		strings.HasPrefix(path, "github.com/will2469/argus/pkg/") ||
		strings.HasPrefix(path, "github.com/will2469/argus/shared/")
}

func isASTContextType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		return sel.Sel.Name == "Context"
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name == "Context"
	}
	return false
}

func isASTStringType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name == "string"
	}
	return false
}

func isASTFuncType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	_, ok := expr.(*ast.FuncType)
	return ok
}

func unwrapPointer(t types.Type) types.Type {
	return dbident.UnwrapPointer(t)
}

func getASTTypeName(expr ast.Expr) string {
	return dbident.GetASTTypeName(expr)
}

func findTypeSpec(name string, file *ast.File) *ast.TypeSpec {
	return dbident.FindTypeSpec(name, file)
}
