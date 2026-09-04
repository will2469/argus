// Package a05_audit_immutability provides path-sensitive query resolution utilities
// to extract candidate SQL statements reaching database calls.
package a05_audit_immutability

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

type flowTracker struct {
	pass       *analysis.Pass
	file       *ast.File
	fn         *ast.FuncDecl
	callStates map[*ast.CallExpr]*flowState
}

func analyzeFunctionFlow(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl) *flowTracker {
	tracker := &flowTracker{
		pass:       pass,
		file:       file,
		fn:         fn,
		callStates: make(map[*ast.CallExpr]*flowState),
	}
	if fn == nil || fn.Body == nil {
		return tracker
	}
	state := newFlowState()
	tracker.analyzeStatements(fn.Body.List, state)
	return tracker
}

func (t *flowTracker) recordCalls(node ast.Node, state *flowState) {
	if node == nil || state == nil {
		return
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			t.callStates[call] = state.clone()
		}
		return true
	})
}

func (t *flowTracker) analyzeStatements(stmts []ast.Stmt, state *flowState) {
	for _, stmt := range stmts {
		t.recordCalls(stmt, state)
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			t.handleAssign(s, state)
		case *ast.DeclStmt:
			t.handleDecl(s, state)
		case *ast.IfStmt:
			t.handleIf(s, state)
		case *ast.SwitchStmt:
			t.handleSwitch(s, state)
		case *ast.ForStmt:
			t.handleFor(s, state)
		case *ast.RangeStmt:
			t.handleRange(s, state)
		case *ast.BlockStmt:
			t.analyzeStatements(s.List, state)
		}

		// Inspect inner function closures
		ast.Inspect(stmt, func(n ast.Node) bool {
			if fnLit, ok := n.(*ast.FuncLit); ok && fnLit.Body != nil {
				closureState := state.clone()
				t.analyzeStatements(fnLit.Body.List, closureState)
				return false
			}
			return true
		})
	}
}

func (t *flowTracker) handleAssign(s *ast.AssignStmt, state *flowState) {
	if s.Tok == token.DEFINE {
		for i, lhs := range s.Lhs {
			lhsId, ok := lhs.(*ast.Ident)
			if !ok || i >= len(s.Rhs) {
				continue
			}
			vals := t.resolveExpr(s.Rhs[i], state, 0)
			k := makeDefVarKey(t.pass, lhsId)
			state.set(k, vals)
		}
	} else if s.Tok == token.ASSIGN {
		for i, lhs := range s.Lhs {
			lhsId, ok := lhs.(*ast.Ident)
			if !ok || i >= len(s.Rhs) {
				continue
			}
			vals := t.resolveExpr(s.Rhs[i], state, 0)
			k := makeVarKey(t.pass, t.file, t.fn, lhsId)
			state.set(k, vals)
		}
	} else if s.Tok == token.ADD_ASSIGN {
		for i, lhs := range s.Lhs {
			lhsId, ok := lhs.(*ast.Ident)
			if !ok || i >= len(s.Rhs) {
				continue
			}
			k := makeVarKey(t.pass, t.file, t.fn, lhsId)
			prevVals, _ := state.get(k)
			rhsVals := t.resolveExpr(s.Rhs[i], state, 0)
			state.set(k, crossConcat(prevVals, rhsVals))
		}
	}
}

func (t *flowTracker) handleDecl(s *ast.DeclStmt, state *flowState) {
	gen, ok := s.Decl.(*ast.GenDecl)
	if !ok || gen.Tok != token.VAR {
		return
	}
	for _, spec := range gen.Specs {
		valSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range valSpec.Names {
			if i < len(valSpec.Values) {
				vals := t.resolveExpr(valSpec.Values[i], state, 0)
				k := makeDefVarKey(t.pass, name)
				state.set(k, vals)
			}
		}
	}
}

func (t *flowTracker) handleIf(s *ast.IfStmt, state *flowState) {
	if s.Init != nil {
		t.analyzeStatements([]ast.Stmt{s.Init}, state)
	}
	t.recordCalls(s.Cond, state)

	thenState := state.clone()
	if s.Body != nil {
		t.analyzeStatements(s.Body.List, thenState)
	}

	var elseState *flowState
	if s.Else != nil {
		elseState = state.clone()
		switch el := s.Else.(type) {
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

	*state = *thenState.join(elseState)
}

func (t *flowTracker) handleSwitch(s *ast.SwitchStmt, state *flowState) {
	if s.Init != nil {
		t.analyzeStatements([]ast.Stmt{s.Init}, state)
	}
	var caseStates []*flowState
	hasDefault := false
	if s.Body != nil {
		for _, clause := range s.Body.List {
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
	merged := newFlowState()
	if !hasDefault {
		caseStates = append(caseStates, state.clone())
	}
	for _, cs := range caseStates {
		merged = merged.join(cs)
	}
	*state = *merged
}

func (t *flowTracker) handleFor(s *ast.ForStmt, state *flowState) {
	if s.Init != nil {
		t.analyzeStatements([]ast.Stmt{s.Init}, state)
	}
	loopState := state.clone()
	if s.Body != nil {
		t.analyzeStatements(s.Body.List, loopState)
	}
	if s.Post != nil {
		t.analyzeStatements([]ast.Stmt{s.Post}, loopState)
	}
	*state = *state.join(loopState)
}

func (t *flowTracker) handleRange(s *ast.RangeStmt, state *flowState) {
	loopState := state.clone()
	if s.Body != nil {
		t.analyzeStatements(s.Body.List, loopState)
	}
	*state = *state.join(loopState)
}

