// Package a08_tx_io provides flow-sensitive control-flow graph and path analysis
// for tracking explicit database transaction lifetimes (Begin ... Commit/Rollback) across branches and loops.
package a08_tx_io

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// InspectExplicitTxFlow analyzes a function body for blocking external I/O executed during explicit transaction lifetimes.
func InspectExplicitTxFlow(pass *analysis.Pass, fset *token.FileSet, fn *ast.FuncDecl, file *ast.File, funcDecls []*ast.FuncDecl, visited map[*ast.FuncDecl]bool, dm *directives.DirectiveMap, issues *[]Issue) {
	if fn == nil || fn.Body == nil {
		return
	}

	walker := &txFlowWalker{
		pass:      pass,
		fset:      fset,
		fn:        fn,
		file:      file,
		funcDecls: funcDecls,
		visited:   visited,
		dm:        dm,
		issues:    issues,
	}

	initialState := newTxState()
	walker.walkBlock(fn.Body, initialState)
}

type txFlowWalker struct {
	pass      *analysis.Pass
	fset      *token.FileSet
	fn        *ast.FuncDecl
	file      *ast.File
	funcDecls []*ast.FuncDecl
	visited   map[*ast.FuncDecl]bool
	dm        *directives.DirectiveMap
	issues    *[]Issue
}

func (w *txFlowWalker) walkBlock(block *ast.BlockStmt, state *TxState) *TxState {
	if block == nil || state.terminated {
		return state
	}

	for _, stmt := range block.List {
		if state.terminated {
			break
		}
		state = w.walkStmt(stmt, state)
	}
	return state
}

func (w *txFlowWalker) walkStmt(stmt ast.Stmt, state *TxState) *TxState {
	if stmt == nil || state.terminated {
		return state
	}

	switch s := stmt.(type) {
	case *ast.AssignStmt:
		return w.handleAssignStmt(s, state)
	case *ast.ExprStmt:
		return w.handleExprStmt(s, state)
	case *ast.IfStmt:
		return w.handleIfStmt(s, state)
	case *ast.ForStmt:
		return w.handleForStmt(s, state)
	case *ast.RangeStmt:
		return w.handleRangeStmt(s, state)
	case *ast.SwitchStmt:
		return w.handleSwitchStmt(s, state)
	case *ast.TypeSwitchStmt:
		return w.handleTypeSwitchStmt(s, state)
	case *ast.BlockStmt:
		return w.walkBlock(s, state)
	case *ast.ReturnStmt:
		if state.hasActive() {
			for _, res := range s.Results {
				w.checkTxExpr(res)
			}
		}
		state.terminated = true
		return state
	case *ast.BranchStmt:
		state.terminated = true
		return state
	case *ast.DeferStmt:
		if state.hasActive() {
			w.checkTxExpr(s.Call)
		}
		return state
	}

	if state.hasActive() {
		w.checkTxExpr(stmt)
	}
	return state
}

func (w *txFlowWalker) handleAssignStmt(assign *ast.AssignStmt, state *TxState) *TxState {
	// 1. Check if RHS begins a transaction: tx, err := pool.Begin(ctx)
	for i, rhs := range assign.Rhs {
		if call, ok := rhs.(*ast.CallExpr); ok && isBeginTxCall(w.pass, call, w.fn, w.file) {
			targetIdx := 0
			if i < len(assign.Lhs) {
				targetIdx = i
			}
			if len(assign.Lhs) > targetIdx {
				if id, ok := assign.Lhs[targetIdx].(*ast.Ident); ok && isDBTxIdent(w.pass, id, w.fn, w.file) {
					k := makeVarKey(w.pass, id, w.fn, w.file)
					state.activate(k)
					return state
				}
			}
		}
	}

	// 2. Check if RHS ends a transaction: err = tx.Commit(ctx)
	for _, rhs := range assign.Rhs {
		if txID := getTxEndIdent(rhs); txID != nil {
			k := makeVarKey(w.pass, txID, w.fn, w.file)
			state.deactivate(k)
			return state
		}
	}

	// 3. If in active transaction, check for blocking I/O on RHS
	if state.hasActive() {
		for _, rhs := range assign.Rhs {
			w.checkTxExpr(rhs)
		}
	}

	return state
}

func (w *txFlowWalker) handleExprStmt(exprStmt *ast.ExprStmt, state *TxState) *TxState {
	if txID := getTxEndIdent(exprStmt.X); txID != nil {
		k := makeVarKey(w.pass, txID, w.fn, w.file)
		state.deactivate(k)
		return state
	}

	if state.hasActive() {
		w.checkTxExpr(exprStmt.X)
	}
	return state
}

func (w *txFlowWalker) handleIfStmt(ifStmt *ast.IfStmt, state *TxState) *TxState {
	if ifStmt.Init != nil {
		state = w.walkStmt(ifStmt.Init, state)
	}

	if state.hasActive() && ifStmt.Cond != nil {
		w.checkTxExpr(ifStmt.Cond)
	}

	thenState := w.walkBlock(ifStmt.Body, state.clone())

	var elseState *TxState
	if ifStmt.Else != nil {
		elseState = w.walkStmt(ifStmt.Else, state.clone())
	} else {
		elseState = state.clone()
	}

	return joinTxStates(thenState, elseState)
}

func (w *txFlowWalker) handleForStmt(forStmt *ast.ForStmt, state *TxState) *TxState {
	if forStmt.Init != nil {
		state = w.walkStmt(forStmt.Init, state)
	}
	if state.hasActive() && forStmt.Cond != nil {
		w.checkTxExpr(forStmt.Cond)
	}

	bodyState := w.walkBlock(forStmt.Body, state.clone())
	return joinTxStates(state, bodyState)
}

func (w *txFlowWalker) handleRangeStmt(rangeStmt *ast.RangeStmt, state *TxState) *TxState {
	if state.hasActive() && rangeStmt.X != nil {
		w.checkTxExpr(rangeStmt.X)
	}

	bodyState := w.walkBlock(rangeStmt.Body, state.clone())
	return joinTxStates(state, bodyState)
}

func (w *txFlowWalker) handleSwitchStmt(switchStmt *ast.SwitchStmt, state *TxState) *TxState {
	if switchStmt.Init != nil {
		state = w.walkStmt(switchStmt.Init, state)
	}
	if state.hasActive() && switchStmt.Tag != nil {
		w.checkTxExpr(switchStmt.Tag)
	}

	return w.walkSwitchClauses(switchStmt.Body.List, state)
}

func (w *txFlowWalker) handleTypeSwitchStmt(ts *ast.TypeSwitchStmt, state *TxState) *TxState {
	if ts.Init != nil {
		state = w.walkStmt(ts.Init, state)
	}
	return w.walkSwitchClauses(ts.Body.List, state)
}

func (w *txFlowWalker) walkSwitchClauses(list []ast.Stmt, state *TxState) *TxState {
	var branchStates []*TxState
	for _, stmt := range list {
		if cc, ok := stmt.(*ast.CaseClause); ok {
			cState := state.clone()
			for _, s := range cc.Body {
				cState = w.walkStmt(s, cState)
			}
			branchStates = append(branchStates, cState)
		}
	}
	if len(branchStates) == 0 {
		return state
	}
	return joinTxStates(branchStates...)
}

func (w *txFlowWalker) checkTxExpr(node ast.Node) {
	if node == nil {
		return
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		CheckTxNode(w.pass, w.fset, n, w.fn, w.file, w.funcDecls, w.visited, w.dm, w.issues)
		return true
	})
}
