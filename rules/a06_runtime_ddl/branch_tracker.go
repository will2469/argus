// Package a06_runtime_ddl provides control-flow branching and loop tracking for DDL detection.
package a06_runtime_ddl

import (
	"go/ast"
)

func (t *DDLTracker) handleIf(stmt *ast.IfStmt, state *ddlState) {
	if stmt.Init != nil {
		t.analyzeStatements([]ast.Stmt{stmt.Init}, state)
	}
	t.recordNodeState(stmt.Cond, state)

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

func (t *DDLTracker) handleSwitch(stmt *ast.SwitchStmt, state *ddlState) {
	if stmt.Init != nil {
		t.analyzeStatements([]ast.Stmt{stmt.Init}, state)
	}
	var caseStates []*ddlState
	hasDefault := false
	if stmt.Body != nil {
		for _, clause := range stmt.Body.List {
			cc, ok := clause.(*ast.CaseClause)
			if !ok {
				continue
			}
			if len(cc.List) == 0 {
				hasDefault = true
			}
			cs := state.clone()
			t.analyzeStatements(cc.Body, cs)
			caseStates = append(caseStates, cs)
		}
	}
	merged := newDDLState()
	if !hasDefault {
		caseStates = append(caseStates, state.clone())
	}
	for _, cs := range caseStates {
		merged = merged.join(cs)
	}
	*state = *merged
}

func (t *DDLTracker) handleFor(stmt *ast.ForStmt, state *ddlState) {
	if stmt.Init != nil {
		t.analyzeStatements([]ast.Stmt{stmt.Init}, state)
	}
	loopState := state.clone()
	if stmt.Body != nil {
		t.analyzeStatements(stmt.Body.List, loopState)
	}
	if stmt.Post != nil {
		t.analyzeStatements([]ast.Stmt{stmt.Post}, loopState)
	}
	*state = *state.join(loopState)
}

func (t *DDLTracker) handleRange(stmt *ast.RangeStmt, state *ddlState) {
	loopState := state.clone()
	if stmt.Body != nil {
		t.analyzeStatements(stmt.Body.List, loopState)
	}
	*state = *state.join(loopState)
}

func (t *DDLTracker) handleCall(call *ast.CallExpr, state *ddlState) {
	method, targetIdent, ok := isSemanticBuilderCall(t.pass, t.currentFile, t.currentFn, call)
	if !ok || targetIdent == nil {
		return
	}

	k := makeVarKey(t.pass, t.currentFile, t.currentFn, targetIdent)
	switch method {
	case "WriteString", "Write":
		if len(call.Args) > 0 {
			if op := t.evalExpr(call.Args[0], state); op != "" {
				state.set(k, ddlValue{kind: ddlKindDefinite, op: op})
			}
		}
	case "Reset":
		state.set(k, ddlValue{kind: ddlKindClean})
	}
}
