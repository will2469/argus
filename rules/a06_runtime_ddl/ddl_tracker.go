// Package a06_runtime_ddl tracks flow-sensitive variable state and DDL provenance.
package a06_runtime_ddl

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// DDLTracker tracks flow-sensitive DDL provenance across functions.
type DDLTracker struct {
	pass       *analysis.Pass
	files      []*ast.File
	currentFn  *ast.FuncDecl
	nodeStates map[ast.Node]*ddlState
}

// NewDDLTracker creates a flow-sensitive DDLTracker.
func NewDDLTracker(pass *analysis.Pass, files ...*ast.File) *DDLTracker {
	fList := files
	if pass != nil {
		fList = pass.Files
	}
	return &DDLTracker{pass: pass, files: fList, nodeStates: make(map[ast.Node]*ddlState)}
}

// SetCurrentFunc sets the active function declaration.
func (t *DDLTracker) SetCurrentFunc(fn *ast.FuncDecl) {
	t.currentFn = fn
}

// Analyze traverses statements and records flow-sensitive DDL states.
func (t *DDLTracker) Analyze() {
	for _, file := range t.files {
		if file == nil {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			t.currentFn = fn
			state := newDDLState()
			t.analyzeStatements(fn.Body.List, state)
		}
	}
	t.currentFn = nil
}

func (t *DDLTracker) recordNodeState(node ast.Node, state *ddlState) {
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

func (t *DDLTracker) analyzeStatements(stmts []ast.Stmt, state *ddlState) {
	for _, stmt := range stmts {
		t.recordNodeState(stmt, state)
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			t.handleAssign(s, state)
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
		case *ast.ReturnStmt:
			for _, res := range s.Results {
				t.recordNodeState(res, state)
			}
		}
	}
}

func (t *DDLTracker) handleAssign(stmt *ast.AssignStmt, state *ddlState) {
	for i, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || i >= len(stmt.Rhs) {
			continue
		}
		rhs := stmt.Rhs[i]
		t.recordNodeState(rhs, state)

		op := t.evalExpr(rhs, state)
		if op != "" {
			state.markDDL(ident, op, t.pass)
		} else if stmt.Tok == token.ASSIGN || stmt.Tok == token.DEFINE {
			state.markClean(ident, t.pass)
		}
	}
}

func (t *DDLTracker) handleIf(stmt *ast.IfStmt, state *ddlState) {
	if assign, ok := stmt.Init.(*ast.AssignStmt); ok {
		t.handleAssign(assign, state)
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

func (t *DDLTracker) handleFor(stmt *ast.ForStmt, state *ddlState) {
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

func (t *DDLTracker) handleRange(stmt *ast.RangeStmt, state *ddlState) {
	loopState := state.clone()
	if stmt.Body != nil {
		t.analyzeStatements(stmt.Body.List, loopState)
	}
	*state = *state.join(loopState)
}

func (t *DDLTracker) handleCall(call *ast.CallExpr, state *ddlState) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}

	switch sel.Sel.Name {
	case "WriteString", "Write":
		if len(call.Args) > 0 {
			if op := t.evalExpr(call.Args[0], state); op != "" {
				state.markDDL(id, op, t.pass)
			}
		}
	case "Reset":
		state.markClean(id, t.pass)
	}
}

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
			return state.getOp(e, t.pass)
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
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "String" {
			if id, ok := sel.X.(*ast.Ident); ok && state != nil {
				return state.getOp(id, t.pass)
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
