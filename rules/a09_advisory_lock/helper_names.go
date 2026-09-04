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

func isKnownNonLockTypeName(name string) bool {
	switch strings.ToLower(name) {
	case "calculator", "searchengine", "search", "player", "videoplayer",
		"audioplayer", "parser", "compiler", "logger", "formatter",
		"math", "cache", "client", "worker", "queue":
		return true
	}
	return false
}

func isApprovedLockHelperTypeName(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "helper", "lockhelper", "advisoryhelper", "advisorylockhelper",
		"locker", "advisorylocker", "distributedlocker",
		"lockmanager", "advisorylockmanager", "txlockmanager",
		"lockservice", "advisorylockservice", "advisorylock",
		"txmanager", "transactionmanager":
		return true
	}
	if strings.HasSuffix(lower, "locker") || strings.HasSuffix(lower, "lockmanager") || strings.HasSuffix(lower, "lockhelper") {
		return true
	}
	return false
}

func hasInvalidTypeOrMethods(t types.Type, methodName string) bool {
	if t == nil {
		return true
	}
	if hasInvalidType(t) {
		return true
	}
	mset := types.NewMethodSet(t)
	if checkMethodHasInvalid(mset, methodName) {
		return true
	}
	if _, isPtr := t.(*types.Pointer); !isPtr {
		msetPtr := types.NewMethodSet(types.NewPointer(t))
		if checkMethodHasInvalid(msetPtr, methodName) {
			return true
		}
	}
	return false
}

func checkMethodHasInvalid(mset *types.MethodSet, methodName string) bool {
	if mset == nil {
		return false
	}
	for i := 0; i < mset.Len(); i++ {
		m := mset.At(i).Obj()
		if m.Name() == methodName {
			if sig, ok := m.Type().(*types.Signature); ok && sig.Params() != nil {
				for j := 0; j < sig.Params().Len(); j++ {
					if hasInvalidType(sig.Params().At(j).Type()) {
						return true
					}
				}
			}
		}
	}
	return false
}

func hasInvalidType(t types.Type) bool {
	return dbident.HasInvalidType(t)
}

func getTypeName(t types.Type) string {
	return dbident.GetTypeName(t)
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
