// Package a06_runtime_ddl tracks flow-sensitive variable state and DDL provenance.
package a06_runtime_ddl

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// DDLTracker tracks flow-sensitive DDL provenance across functions.
type DDLTracker struct {
	pass        *analysis.Pass
	files       []*ast.File
	currentFile *ast.File
	currentFn   *ast.FuncDecl
	nodeStates  map[ast.Node]*ddlState
}

// NewDDLTracker creates a flow-sensitive DDLTracker.
func NewDDLTracker(pass *analysis.Pass, files ...*ast.File) *DDLTracker {
	fList := files
	if pass != nil {
		fList = pass.Files
	}
	return &DDLTracker{pass: pass, files: fList, nodeStates: make(map[ast.Node]*ddlState)}
}

// SetCurrentFunc sets the active function declaration and enclosing file.
func (t *DDLTracker) SetCurrentFunc(fn *ast.FuncDecl) {
	t.currentFn = fn
}

// Analyze traverses statements and records flow-sensitive DDL states.
func (t *DDLTracker) Analyze() {
	for _, file := range t.files {
		if file == nil {
			continue
		}
		t.currentFile = file
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
		case *ast.DeclStmt:
			t.handleDecl(s, state)
		case *ast.IfStmt:
			t.handleIf(s, state)
		case *ast.SwitchStmt:
			t.handleSwitch(s, state)
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

func (t *DDLTracker) handleAssign(stmt *ast.AssignStmt, state *ddlState) {
	if stmt.Tok == token.DEFINE {
		for i, lhs := range stmt.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok || i >= len(stmt.Rhs) {
				continue
			}
			rhs := stmt.Rhs[i]
			t.recordNodeState(rhs, state)
			k := makeDefVarKey(t.pass, ident)
			op := t.evalExpr(rhs, state)
			if op != "" {
				state.set(k, ddlValue{kind: ddlKindDefinite, op: op})
			} else {
				state.set(k, ddlValue{kind: ddlKindClean})
			}
		}
	} else if stmt.Tok == token.ASSIGN {
		for i, lhs := range stmt.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok || i >= len(stmt.Rhs) {
				continue
			}
			rhs := stmt.Rhs[i]
			t.recordNodeState(rhs, state)
			k := makeVarKey(t.pass, t.currentFile, t.currentFn, ident)
			op := t.evalExpr(rhs, state)
			if op != "" {
				state.set(k, ddlValue{kind: ddlKindDefinite, op: op})
			} else {
				state.set(k, ddlValue{kind: ddlKindClean})
			}
		}
	} else if stmt.Tok == token.ADD_ASSIGN {
		for i, lhs := range stmt.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok || i >= len(stmt.Rhs) {
				continue
			}
			rhs := stmt.Rhs[i]
			t.recordNodeState(rhs, state)
			k := makeVarKey(t.pass, t.currentFile, t.currentFn, ident)
			if op := t.evalExpr(rhs, state); op != "" {
				state.set(k, ddlValue{kind: ddlKindDefinite, op: op})
			}
		}
	}
}

func (t *DDLTracker) handleDecl(stmt *ast.DeclStmt, state *ddlState) {
	gen, ok := stmt.Decl.(*ast.GenDecl)
	if !ok || gen.Tok != token.VAR {
		return
	}
	for _, spec := range gen.Specs {
		valSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range valSpec.Names {
			k := makeDefVarKey(t.pass, name)
			if i < len(valSpec.Values) {
				op := t.evalExpr(valSpec.Values[i], state)
				if op != "" {
					state.set(k, ddlValue{kind: ddlKindDefinite, op: op})
				} else {
					state.set(k, ddlValue{kind: ddlKindClean})
				}
			} else {
				state.set(k, ddlValue{kind: ddlKindClean})
			}
		}
	}
}


