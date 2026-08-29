// Package a22_serializable_retry tracks transaction options and isolation levels for ARGUS-A22.
package a22_serializable_retry

import (
	"go/ast"
	"strings"
)

// IsStrictIsolationLevel checks if the expression or TxOptions represents Serializable or RepeatableRead.
func IsStrictIsolationLevel(expr ast.Expr) bool {
	if expr == nil {
		return false
	}

	switch e := expr.(type) {
	case *ast.CompositeLit:
		for _, elt := range e.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if keyIdent, ok := kv.Key.(*ast.Ident); ok && keyIdent.Name == "IsoLevel" {
					return isStrictLevelVal(kv.Value)
				}
			}
		}
	case *ast.Ident:
		if e.Obj != nil {
			if assign, ok := e.Obj.Decl.(*ast.AssignStmt); ok {
				for _, r := range assign.Rhs {
					if IsStrictIsolationLevel(r) {
						return true
					}
				}
			}
			if spec, ok := e.Obj.Decl.(*ast.ValueSpec); ok {
				for _, v := range spec.Values {
					if IsStrictIsolationLevel(v) {
						return true
					}
				}
			}
		}
	case *ast.SelectorExpr:
		return isStrictLevelVal(e)
	}

	return false
}

func isStrictLevelVal(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		name := e.Sel.Name
		return name == "Serializable" || name == "RepeatableRead" || name == "LevelSerializable" || name == "LevelRepeatableRead"
	case *ast.Ident:
		name := strings.ToLower(e.Name)
		return strings.Contains(name, "serializable") || strings.Contains(name, "repeatableread")
	case *ast.BasicLit:
		val := strings.ToLower(strings.Trim(e.Value, `"'`))
		return val == "serializable" || val == "repeatable read" || val == "repeatable_read"
	}
	return false
}
