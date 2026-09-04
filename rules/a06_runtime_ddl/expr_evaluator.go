// Package a06_runtime_ddl provides AST expression evaluation utilities for DDL detection.
package a06_runtime_ddl

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

func (t *DDLTracker) evalExpr(expr ast.Expr, state *ddlState) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			val, err := strconv.Unquote(e.Value)
			if err != nil {
				val = strings.Trim(e.Value, "`\"")
			}
			if op := DetectDDLFromAST(val); op != "" {
				return op
			}
			return MatchDDLCommand(val)
		}
	case *ast.Ident:
		if state != nil {
			k := makeVarKey(t.pass, t.currentFile, t.currentFn, e)
			if v, ok := state.get(k); ok {
				return v.getOp()
			}
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			if op := t.evalExpr(e.X, state); op != "" {
				return op
			}
			if op := t.evalExpr(e.Y, state); op != "" {
				return op
			}
			return evalConcatDDL(e)
		}
	case *ast.CallExpr:
		if method, targetIdent, ok := isSemanticBuilderCall(t.pass, t.currentFile, t.currentFn, e); ok && method == "String" {
			if state != nil {
				k := makeVarKey(t.pass, t.currentFile, t.currentFn, targetIdent)
				if v, ok := state.get(k); ok {
					return v.getOp()
				}
			}
		}
		return EvalDynamicDDL(e)
	case *ast.ParenExpr:
		return t.evalExpr(e.X, state)
	}

	return ""
}

// GetDDLOpAt returns any DDL operation associated with expr at the given node.
func (t *DDLTracker) GetDDLOpAt(expr ast.Expr, at ast.Node) string {
	if t == nil || expr == nil {
		return ""
	}
	var state *ddlState
	if at != nil {
		state = t.nodeStates[at]
	}
	if state == nil {
		state = newDDLState()
	}
	return t.evalExpr(expr, state)
}
