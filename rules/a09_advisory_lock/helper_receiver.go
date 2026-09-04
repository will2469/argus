// Package a09_advisory_lock provides semantic and AST proof for advisory lock helper receivers,
// ensuring only approved packages, lock managers, and valid helper signatures are inspected.
package a09_advisory_lock

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// isAdvisoryLockHelperReceiver verifies whether a selector expression refers to an approved
// advisory lock helper using semantic type verification and fail-closed AST fallback.
func isAdvisoryLockHelperReceiver(pass *analysis.Pass, sel *ast.SelectorExpr, fn *ast.FuncDecl, file *ast.File) bool {
	if sel == nil {
		return false
	}

	// 1. Check if sel.X is an approved package (e.g. argus.WithAdvisoryLock)
	if id, ok := sel.X.(*ast.Ident); ok {
		if isApprovedPackageCall(pass, id, fn, file) {
			return true
		}
	}

	// 2. Semantic Type Checking via pass.TypesInfo (when available)
	if pass != nil && pass.TypesInfo != nil {
		var recvType types.Type
		if selType, ok := pass.TypesInfo.Selections[sel]; ok && selType.Recv() != nil {
			recvType = selType.Recv()
		} else if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.Type != nil {
			recvType = tv.Type
		} else if id, ok := sel.X.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				recvType = obj.Type()
			} else if obj := pass.TypesInfo.Defs[id]; obj != nil {
				recvType = obj.Type()
			}
		}

		if recvType != nil && recvType != types.Typ[types.Invalid] {
			if isProvenLockHelperType(recvType, sel.Sel.Name) {
				return true
			}
			// Reject known non-lock types immediately (anti-false-positive boundary)
			if isKnownNonLockTypeName(getTypeName(recvType)) {
				return false
			}
			// Only fall through to AST if type information contains invalid/unresolved symbols
			if !hasInvalidTypeOrMethods(recvType, sel.Sel.Name) {
				return false
			}
		}
	}

	// 3. AST-only fallback: Resolve receiver type and verify contract
	astType := findASTReceiverType(sel.X, fn, file)
	if astType != nil {
		typeName := getASTTypeName(astType)
		if isKnownNonLockTypeName(typeName) {
			return false
		}
		if isApprovedLockHelperTypeName(typeName) {
			return hasASTProvenLockMethod(typeName, sel.Sel.Name, file)
		}
	}

	// Fail-closed against false positives: unproven receivers are rejected
	return false
}

func isApprovedPackageCall(pass *analysis.Pass, id *ast.Ident, fn *ast.FuncDecl, file *ast.File) bool {
	if id == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Uses[id]; obj != nil {
			if pkg, ok := obj.(*types.PkgName); ok {
				return isArgusPackagePath(pkg.Imported().Path())
			}
		}
		return false
	}
	if file != nil {
		if isImportedArgusPackage(file, id.Name) && !isIdentShadowed(file, fn, id.Pos(), id.Name) {
			return true
		}
	}
	return false
}

func isProvenLockHelperType(t types.Type, methodName string) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)

	// Named types verification
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		typeName := named.Obj().Name()
		// Always reject known non-lock types (e.g. Calculator, SearchEngine)
		if isKnownNonLockTypeName(typeName) {
			return false
		}
		pkg := named.Obj().Pkg()
		if pkg != nil && isArgusPackagePath(pkg.Path()) {
			return hasProvenAdvisoryLockMethod(t, methodName)
		}
		if !isApprovedLockHelperTypeName(typeName) {
			return false
		}
	}

	return hasProvenAdvisoryLockMethod(t, methodName)
}

func hasProvenAdvisoryLockMethod(t types.Type, methodName string) bool {
	if t == nil {
		return false
	}
	mset := types.NewMethodSet(t)
	if checkMethodSet(mset, methodName) {
		return true
	}
	if _, isPtr := t.(*types.Pointer); !isPtr {
		msetPtr := types.NewMethodSet(types.NewPointer(t))
		if checkMethodSet(msetPtr, methodName) {
			return true
		}
	}
	return false
}

func checkMethodSet(mset *types.MethodSet, methodName string) bool {
	if mset == nil {
		return false
	}
	for i := 0; i < mset.Len(); i++ {
		m := mset.At(i).Obj()
		if m.Name() == methodName {
			sig, ok := m.Type().(*types.Signature)
			if ok && isProvenAdvisoryLockSignature(methodName, sig) {
				return true
			}
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
		if !isContextType(params.At(0).Type()) || !isStringType(params.At(2).Type()) {
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
		if !isContextType(params.At(0).Type()) || !isStringType(params.At(2).Type()) {
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
		return isContextType(params.At(0).Type()) && isStringType(params.At(2).Type())
	}
	return false
}

func isContextType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		if named.Obj().Name() == "Context" {
			return true
		}
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			if iface.Method(i).Name() == "Done" {
				return true
			}
		}
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
