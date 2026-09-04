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

func (t *flowTracker) resolveExpr(expr ast.Expr, state *flowState, depth int) valueSet {
	if depth > 5 || expr == nil {
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
		if vs, ok := state.get(k); ok && len(vs) > 0 {
			return vs.clone()
		}
		return t.findPackageDeclValues(e.Name, depth+1)
	}
	return nil
}

func (t *flowTracker) findPackageDeclValues(name string, depth int) valueSet {
	if t.file == nil {
		return nil
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
					return t.resolveExpr(valSpec.Values[i], newFlowState(), depth)
				}
			}
		}
	}
	return nil
}

func crossConcat(left, right valueSet) valueSet {
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
		return nil
	}

	state := t.callStates[call]
	if state == nil {
		state = newFlowState()
	}

	vs := t.resolveExpr(sqlArg, state, 0)
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
