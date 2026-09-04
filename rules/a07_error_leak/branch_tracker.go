// Package a07_error_leak provides control-flow branching and loop convergence handlers for ErrorTracker.
package a07_error_leak

import (
	"go/ast"
)

func (t *ErrorTracker) handleIf(stmt *ast.IfStmt, state *errorState) {
	if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
		t.handleAssign(assign, state)
	}

	thenState := state.clone()
	if stmt.Body != nil {
		t.analyzeStatements(stmt.Body.List, thenState)
	}

	elseState := state.clone()
	if stmt.Else != nil {
		switch el := stmt.Else.(type) {
		case *ast.BlockStmt:
			t.analyzeStatements(el.List, elseState)
		case *ast.IfStmt:
			t.handleIf(el, elseState)
		default:
			t.analyzeStatements([]ast.Stmt{el}, elseState)
		}
	}

	*state = *thenState.join(elseState)
}

func (t *ErrorTracker) handleSwitch(stmt *ast.SwitchStmt, state *errorState) {
	if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
		t.handleAssign(assign, state)
	}
	if stmt.Body == nil || len(stmt.Body.List) == 0 {
		return
	}

	joined := newErrorState()
	for _, cl := range stmt.Body.List {
		cc, ok := cl.(*ast.CaseClause)
		if !ok {
			continue
		}
		caseState := state.clone()
		t.analyzeStatements(cc.Body, caseState)
		joined = joined.join(caseState)
	}
	*state = *joined
}

func (t *ErrorTracker) handleFor(stmt *ast.ForStmt, state *errorState) {
	if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
		t.handleAssign(assign, state)
	}
	loopState := state.clone()
	if stmt.Body != nil {
		t.analyzeStatements(stmt.Body.List, loopState)
	}
	if assign, ok := stmt.Post.(*ast.AssignStmt); ok {
		t.handleAssign(assign, loopState)
	}
	*state = *state.join(loopState)
}

func (t *ErrorTracker) handleRange(stmt *ast.RangeStmt, state *errorState) {
	loopState := state.clone()
	if stmt.Body != nil {
		t.analyzeStatements(stmt.Body.List, loopState)
	}
	*state = *state.join(loopState)
}
