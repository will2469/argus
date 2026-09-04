// Package a10_isolation_level provides lexical object identity and constant resolution
// preventing variable shadowing from breaking transaction and query provenance.
package a10_isolation_level

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// isSameObject verifies whether two identifier AST nodes refer to the exact same types.Object or ast.Object,
// preventing shadowing bugs where an inner variable inherits an outer variable's proof.
func isSameObject(pass *analysis.Pass, lhsID, targetID *ast.Ident) bool {
	if lhsID == nil || targetID == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		targetObj := pass.TypesInfo.ObjectOf(targetID)
		if targetObj == nil {
			targetObj = pass.TypesInfo.Uses[targetID]
		}
		if targetObj == nil {
			targetObj = pass.TypesInfo.Defs[targetID]
		}

		lhsObj := pass.TypesInfo.Defs[lhsID]
		if lhsObj == nil {
			lhsObj = pass.TypesInfo.Uses[lhsID]
		}
		if lhsObj == nil {
			lhsObj = pass.TypesInfo.ObjectOf(lhsID)
		}

		if targetObj != nil && lhsObj != nil {
			return targetObj == lhsObj
		}
	}

	if targetID.Obj != nil && lhsID.Obj != nil {
		return targetID.Obj == lhsID.Obj
	}

	return targetID.Name == lhsID.Name
}

// resolveStringConstant extracts the constant string value if the identifier refers to a string constant.
func resolveStringConstant(pass *analysis.Pass, id *ast.Ident) (string, bool) {
	if id == nil {
		return "", false
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
			if c, ok := obj.(*types.Const); ok && c.Val().Kind() == constant.String {
				return constant.StringVal(c.Val()), true
			}
		}
	}
	if id.Obj != nil && id.Obj.Kind == ast.Con {
		if vs, ok := id.Obj.Decl.(*ast.ValueSpec); ok {
			for _, val := range vs.Values {
				if lit, ok := val.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					return strings.Trim(lit.Value, "`\""), true
				}
			}
		}
	}
	return "", false
}
