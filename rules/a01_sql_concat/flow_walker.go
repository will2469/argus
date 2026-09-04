package a01_sql_concat

import (
	"go/ast"
	"go/token"
)

func (t *TaintTracker) recordNodeState(node ast.Node, state *taintState) {
	if node == nil || state == nil {
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

func (t *TaintTracker) analyzeStatements(stmts []ast.Stmt, state *taintState) {
	for _, stmt := range stmts {
		t.recordNodeState(stmt, state)
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			t.handleAssign(s, state)
			t.analyzeFuncLitsInNode(s, state)
		case *ast.IfStmt:
			t.handleIf(s, state)
		case *ast.BlockStmt:
			t.analyzeStatements(s.List, state)
		case *ast.ForStmt:
			t.handleFor(s, state)
		case *ast.RangeStmt:
			t.handleRange(s, state)
		case *ast.ExprStmt:
			t.recordNodeState(s.X, state)
			if call, ok := s.X.(*ast.CallExpr); ok {
				t.handleCall(call, state)
			}
			t.analyzeFuncLitsInNode(s, state)
		case *ast.GoStmt:
			t.analyzeFuncLitsInNode(s.Call, state)
		case *ast.DeferStmt:
			t.analyzeFuncLitsInNode(s.Call, state)
		case *ast.ReturnStmt:
			for _, res := range s.Results {
				t.recordNodeState(res, state)
				t.analyzeFuncLitsInNode(res, state)
			}
		}
	}
}

func (t *TaintTracker) analyzeFuncLitsInNode(node ast.Node, state *taintState) {
	if node == nil || state == nil {
		return
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if fnLit, ok := n.(*ast.FuncLit); ok && fnLit.Body != nil {
			closureState := state.clone()
			t.analyzeStatements(fnLit.Body.List, closureState)
			return false
		}
		return true
	})
}

func (t *TaintTracker) handleAssign(stmt *ast.AssignStmt, state *taintState) {
	tainted := false
	for _, rhs := range stmt.Rhs {
		t.recordNodeState(rhs, state)
		if t.isExprTaintedInState(rhs, state) {
			tainted = true
			break
		}
	}
	if stmt.Tok == token.ADD_ASSIGN {
		for _, lhs := range stmt.Lhs {
			if t.isExprTaintedInState(lhs, state) {
				tainted = true
				break
			}
		}
	}

	for _, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		if tainted {
			state.markTainted(ident, t.pass)
		} else if stmt.Tok == token.ASSIGN || stmt.Tok == token.DEFINE {
			state.markClean(ident, t.pass)
		}
	}
}

func (t *TaintTracker) handleIf(stmt *ast.IfStmt, state *taintState) {
	if stmt.Init != nil {
		if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
			t.handleAssign(assign, state)
		}
	}
	t.recordNodeState(stmt.Cond, state)

	thenState := state.clone()
	if stmt.Body != nil {
		t.analyzeStatements(stmt.Body.List, thenState)
	}

	var elseState *taintState
	if stmt.Else != nil {
		elseState = state.clone()
		switch el := stmt.Else.(type) {
		case *ast.BlockStmt:
			t.analyzeStatements(el.List, elseState)
		case *ast.IfStmt:
			t.handleIf(el, elseState)
		default:
			t.analyzeStatements([]ast.Stmt{el}, elseState)
		}
	} else {
		elseState = state.clone()
	}

	merged := thenState.join(elseState)
	*state = *merged
}

func (t *TaintTracker) handleFor(stmt *ast.ForStmt, state *taintState) {
	if stmt.Init != nil {
		if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
			t.handleAssign(assign, state)
		}
	}
	loopState := state.clone()
	if stmt.Body != nil {
		t.analyzeStatements(stmt.Body.List, loopState)
	}
	if stmt.Post != nil {
		if assign, ok := stmt.Post.(*ast.AssignStmt); ok {
			t.handleAssign(assign, loopState)
		}
	}
	merged := state.join(loopState)
	*state = *merged
}

func (t *TaintTracker) handleRange(stmt *ast.RangeStmt, state *taintState) {
	loopState := state.clone()
	if t.isExprTaintedInState(stmt.X, state) {
		if val, ok := stmt.Value.(*ast.Ident); ok {
			loopState.markTainted(val, t.pass)
		}
	}
	if stmt.Body != nil {
		t.analyzeStatements(stmt.Body.List, loopState)
	}
	merged := state.join(loopState)
	*state = *merged
}

func (t *TaintTracker) handleCall(call *ast.CallExpr, state *taintState) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	switch sel.Sel.Name {
	case "WriteString", "Write":
		if len(call.Args) > 0 && t.isExprTaintedInState(call.Args[0], state) {
			if ident, ok := sel.X.(*ast.Ident); ok {
				state.markTainted(ident, t.pass)
			}
		}
	case "Reset":
		if ident, ok := sel.X.(*ast.Ident); ok {
			state.markClean(ident, t.pass)
		}
	}
}
