// Package a03_context provides flow-sensitive and scope-aware tracking of context values.
package a03_context

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// contextTracker manages flow-sensitive tracking over AST functions.
type contextTracker struct {
	pass       *analysis.Pass
	file       *ast.File
	nodeStates map[ast.Node]*flowState
}

func newContextTracker(pass *analysis.Pass, file *ast.File) *contextTracker {
	return &contextTracker{
		pass:       pass,
		file:       file,
		nodeStates: make(map[ast.Node]*flowState),
	}
}

func (t *contextTracker) analyzeFunc(fn *ast.FuncDecl) {
	if fn == nil || fn.Body == nil {
		return
	}
	state := newFlowState()
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				var obj types.Object
				if t.pass != nil && t.pass.TypesInfo != nil {
					obj = t.pass.TypesInfo.Defs[name]
				}
				state.define(name.Name, obj, ctxBounded)
			}
		}
	}
	t.analyzeBlock(fn.Body, state)
}

func (t *contextTracker) analyzeFuncLit(lit *ast.FuncLit, parentState *flowState) {
	if lit == nil || lit.Body == nil {
		return
	}
	state := parentState.clone()
	if lit.Type.Params != nil {
		for _, field := range lit.Type.Params.List {
			for _, name := range field.Names {
				var obj types.Object
				if t.pass != nil && t.pass.TypesInfo != nil {
					obj = t.pass.TypesInfo.Defs[name]
				}
				state.define(name.Name, obj, ctxBounded)
			}
		}
	}
	t.analyzeBlock(lit.Body, state)
}

func (t *contextTracker) analyzeBlock(block *ast.BlockStmt, state *flowState) {
	if block == nil {
		return
	}
	state.pushScope()
	defer state.popScope()

	for _, stmt := range block.List {
		t.recordState(stmt, state)
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			t.handleAssign(s, state)
		case *ast.IfStmt:
			t.handleIf(s, state)
		case *ast.BlockStmt:
			t.analyzeBlock(s, state)
		case *ast.ForStmt:
			if s.Body != nil {
				t.analyzeBlock(s.Body, state)
			}
		case *ast.RangeStmt:
			if s.Body != nil {
				t.analyzeBlock(s.Body, state)
			}
		}
	}
}

func (t *contextTracker) handleAssign(stmt *ast.AssignStmt, state *flowState) {
	for i, lhs := range stmt.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		var rhs ast.Expr
		if i < len(stmt.Rhs) {
			rhs = stmt.Rhs[i]
		} else if len(stmt.Rhs) == 1 {
			rhs = stmt.Rhs[0]
		}
		if rhs == nil {
			continue
		}

		var obj types.Object
		if t.pass != nil && t.pass.TypesInfo != nil {
			obj = t.pass.TypesInfo.Defs[id]
			if obj == nil {
				obj = t.pass.TypesInfo.Uses[id]
			}
		}

		kind := t.evalExprKind(rhs, state)
		if stmt.Tok.String() == ":=" {
			state.define(id.Name, obj, kind)
		} else {
			state.update(id.Name, obj, kind)
		}
	}
}

func (t *contextTracker) handleIf(stmt *ast.IfStmt, state *flowState) {
	if stmt.Init != nil {
		if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
			t.handleAssign(assign, state)
		}
	}
	thenState := state.clone()
	if stmt.Body != nil {
		t.analyzeBlock(stmt.Body, thenState)
	}

	var elseState *flowState
	if stmt.Else != nil {
		elseState = state.clone()
		switch el := stmt.Else.(type) {
		case *ast.BlockStmt:
			t.analyzeBlock(el, elseState)
		case *ast.IfStmt:
			t.handleIf(el, elseState)
		}
	} else {
		elseState = state.clone()
	}

	merged := joinStates(thenState, elseState)
	*state = *merged
}

func (t *contextTracker) evalExprKind(e ast.Expr, state *flowState) contextKind {
	switch x := e.(type) {
	case *ast.CallExpr:
		if isRawContextCall(t.pass, t.file, x) {
			return ctxRaw
		}
		if isBoundedContextCall(t.pass, t.file, x) {
			return ctxBounded
		}
	case *ast.Ident:
		var obj types.Object
		if t.pass != nil && t.pass.TypesInfo != nil {
			obj = t.pass.TypesInfo.Uses[x]
		}
		return state.lookup(x.Name, obj)
	}
	return ctxUnknown
}

func (t *contextTracker) recordState(node ast.Node, state *flowState) {
	if node == nil {
		return
	}
	t.nodeStates[node] = state.clone()
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			t.nodeStates[call] = state.clone()
		}
		return true
	})
}

func (t *contextTracker) isExprRaw(e ast.Expr, at ast.Node) bool {
	if e == nil {
		return false
	}
	if call, ok := e.(*ast.CallExpr); ok {
		return isRawContextCall(t.pass, t.file, call)
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	var state *flowState
	if at != nil {
		state = t.nodeStates[at]
	}
	if state == nil {
		return false
	}
	var obj types.Object
	if t.pass != nil && t.pass.TypesInfo != nil {
		obj = t.pass.TypesInfo.Uses[id]
	}
	return state.lookup(id.Name, obj) == ctxRaw
}
