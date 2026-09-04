// Package a09_advisory_lock provides semantic and AST proof for advisory lock helper receivers,
// ensuring only approved packages, lock managers, and valid helper signatures are inspected.
package a09_advisory_lock

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/dbident"
)

// isAdvisoryLockHelperReceiver verifies whether a selector expression refers to an approved
// advisory lock helper using exact types.Func declaration identity and fail-closed AST fallback.
func isAdvisoryLockHelperReceiver(pass *analysis.Pass, sel *ast.SelectorExpr, fn *ast.FuncDecl, file *ast.File) bool {
	if sel == nil {
		return false
	}

	// 1. Semantic Type Checking via pass.TypesInfo (when available):
	// callsite -> exact types.Func / types.Object -> exact approved helper declaration.
	if pass != nil && pass.TypesInfo != nil {
		// A. Method selection on receiver: e.g. h.WithAdvisoryLock(...)
		if selType, ok := pass.TypesInfo.Selections[sel]; ok {
			if fnObj, ok := selType.Obj().(*types.Func); ok {
				return isApprovedHelperFunc(fnObj)
			}
		}

		// B. Package-level function or direct identifier use: e.g. argus.WithAdvisoryLock(...)
		if obj := pass.TypesInfo.Uses[sel.Sel]; obj != nil {
			if fnObj, ok := obj.(*types.Func); ok {
				return isApprovedHelperFunc(fnObj)
			}
		}

		// When compiler types are available but did not resolve to an approved helper, fail closed.
		return false
	}

	// 2. AST-only fallback for standalone CLI runner (pass == nil)
	// A. Check if sel.X is an imported approved Argus package
	if id, ok := sel.X.(*ast.Ident); ok {
		if isImportedArgusPackage(file, id.Name) && !isIdentShadowed(file, fn, id.Pos(), id.Name) {
			return true
		}
	}

	// B. Check if receiver is local Helper struct in internal Argus test fixture
	if file != nil && file.Name != nil && isArgusTestPackage(file.Name.Name) {
		astType := findASTReceiverType(sel.X, fn, file)
		if astType != nil && getASTTypeName(astType) == "Helper" {
			return hasASTProvenLockMethod("Helper", sel.Sel.Name, file)
		}
	}

	// Fail-closed against false positives: unproven receivers are rejected
	return false
}

func isApprovedHelperFunc(fnObj *types.Func) bool {
	if fnObj == nil || fnObj.Pkg() == nil {
		return false
	}

	// Method or function must be an approved helper name
	switch fnObj.Name() {
	case "WithAdvisoryLock", "ExecuteLockedTx", "TryAdvisoryLock":
	default:
		return false
	}

	sig, ok := fnObj.Type().(*types.Signature)
	if !ok || !isProvenAdvisoryLockSignature(fnObj.Name(), sig) {
		return false
	}

	// Path 1: Declared in an approved Argus package
	if isArgusPackagePath(fnObj.Pkg().Path()) {
		return true
	}

	// Path 2: Declared in internal Argus test suite fixture on exact "Helper" type
	pkgPath := fnObj.Pkg().Path()
	if isArgusTestPackage(pkgPath) && sig.Recv() != nil {
		recvNamed, ok := dbident.UnwrapPointer(sig.Recv().Type()).(*types.Named)
		if ok && recvNamed.Obj() != nil && recvNamed.Obj().Name() == "Helper" {
			return true
		}
	}

	return false
}

func isProvenAdvisoryLockSignature(methodName string, sig *types.Signature) bool {
	if sig == nil || sig.Params() == nil {
		return false
	}
	params := sig.Params()
	switch methodName {
	case "WithAdvisoryLock":
		if params.Len() < 3 {
			return false
		}
		if !dbident.IsExactContextType(params.At(0).Type()) || !isStringType(params.At(2).Type()) {
			return false
		}
		if params.Len() >= 4 {
			hasFunc := false
			for j := 3; j < params.Len(); j++ {
				if isFuncType(params.At(j).Type()) {
					hasFunc = true
					break
				}
			}
			if !hasFunc {
				return false
			}
		}
		return true
	case "ExecuteLockedTx":
		if params.Len() < 3 {
			return false
		}
		if !dbident.IsExactContextType(params.At(0).Type()) || !isStringType(params.At(2).Type()) {
			return false
		}
		for j := 2; j < params.Len(); j++ {
			if isFuncType(params.At(j).Type()) {
				return true
			}
		}
		return false
	case "TryAdvisoryLock":
		if params.Len() < 3 {
			return false
		}
		return dbident.IsExactContextType(params.At(0).Type()) && isStringType(params.At(2).Type())
	}
	return false
}

func isStringType(t types.Type) bool {
	if t == nil {
		return false
	}
	if basic, ok := t.Underlying().(*types.Basic); ok {
		return basic.Kind() == types.String
	}
	return false
}

func isFuncType(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Signature)
	return ok
}
