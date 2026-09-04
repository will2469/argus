// Package a05_audit_immutability provides expression evaluation utilities
// to resolve candidate SQL strings from AST expressions.
package a05_audit_immutability

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/will2469/argus/shared/callsite"
)

const maxResolveDepth = 25

func (t *flowTracker) resolveExpr(expr ast.Expr, state *flowState, depth int) valueSet {
	if depth > maxResolveDepth || expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return newValueSet(unquoteLiteral(e.Value))
		}
	case *ast.ParenExpr:
		return t.resolveExpr(e.X, state, depth)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			left := t.resolveExpr(e.X, state, depth+1)
			right := t.resolveExpr(e.Y, state, depth+1)
			return crossConcat(left, right)
		}
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Sprintf" {
			if len(e.Args) > 0 {
				return t.resolveExpr(e.Args[0], state, depth+1)
			}
		}
	case *ast.Ident:
		if t.pass != nil && t.pass.TypesInfo != nil {
			if c, ok := t.pass.TypesInfo.Uses[e].(*types.Const); ok {
				return newValueSet(unquoteLiteral(c.Val().ExactString()))
			}
		}
		k := makeVarKey(t.pass, t.file, t.fn, e)
		if vs, ok := state.get(k); ok {
			// Return nil (Unknown) if the variable state is nil
			// Return empty set if vs is empty (proven empty) - this is valid (no queries)
			if vs == nil {
				return nil
			}
			return vs.clone()
		}
		// Variable not found in state - try package-level fallback
		pkgVals := t.findPackageDeclValues(e.Name, depth+1)
		if pkgVals == nil {
			// Package-level fallback also failed - Unknown (not provably safe)
			return nil
		}
		return pkgVals
	}
	return nil
}

func (t *flowTracker) findPackageDeclValues(name string, depth int) valueSet {
	if t.file == nil {
		return nil // Unknown - no file context
	}
	for _, decl := range t.file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			valSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, n := range valSpec.Names {
				if n.Name == name && i < len(valSpec.Values) {
					// Resolve the expression with a fresh empty state
					// If it returns nil, that's Unknown (not provably safe)
					return t.resolveExpr(valSpec.Values[i], newFlowState(), depth)
				}
			}
		}
	}
	return nil // Unknown - declaration not found
}

func crossConcat(left, right valueSet) valueSet {
	// Fail-closed: if either side is Unknown (nil), result is Unknown
	if left == nil || right == nil {
		return nil
	}
	// Empty sets should result in empty set (proven empty)
	if len(left) == 0 && len(right) == 0 {
		return valueSet{}
	}
	if len(left) == 0 {
		return right.clone()
	}
	if len(right) == 0 {
		return left.clone()
	}
	res := make(valueSet)
	for l := range left {
		for r := range right {
			res[l+r] = struct{}{}
		}
	}
	return res
}

func (t *flowTracker) ResolveCallQueries(call *ast.CallExpr) []string {
	if call == nil {
		return nil
	}
	sqlArg := callsite.ExtractSQLArg(call, t.pass)
	if sqlArg == nil {
		return nil // No SQL argument found
	}

	state := t.callStates[call]
	if state == nil {
		state = newFlowState()
	}

	vs := t.resolveExpr(sqlArg, state, 0)
	// nil valueSet means Unknown (not provably safe) - fail-closed
	// Return empty slice []string{} to distinguish from nil (no SQL arg)
	if vs == nil {
		return []string{} // Unknown provenance - not provably safe
	}
	return vs.toSlice()
}

func unquoteLiteral(val string) string {
	s, err := strconv.Unquote(val)
	if err == nil {
		return s
	}
	if len(val) >= 2 && ((val[0] == '`' && val[len(val)-1] == '`') ||
		(val[0] == '"' && val[len(val)-1] == '"')) {
		return val[1 : len(val)-1]
	}
	return val
}
