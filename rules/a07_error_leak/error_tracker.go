// Package a07_error_leak tracks flow-sensitive error provenance across functions,
// joining abstract states at control-flow convergence points.
package a07_error_leak

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// ErrorTracker performs flow-sensitive error taint analysis across function bodies.
type ErrorTracker struct {
	pass        *analysis.Pass
	files       []*ast.File
	currentFile *ast.File
	currentFn   *ast.FuncDecl
	nodeStates  map[ast.Node]*errorState
	fnStates    map[*ast.FuncDecl]*errorState
}

// NewErrorTracker initializes an ErrorTracker.
func NewErrorTracker(pass *analysis.Pass, files ...*ast.File) *ErrorTracker {
	var fList []*ast.File
	for _, f := range files {
		if f != nil {
			fList = append(fList, f)
		}
	}
	if len(fList) == 0 && pass != nil {
		fList = pass.Files
	}
	return &ErrorTracker{
		pass:       pass,
		files:      fList,
		nodeStates: make(map[ast.Node]*errorState),
		fnStates:   make(map[*ast.FuncDecl]*errorState),
	}
}

// Analyze traverses all functions across files and tracks error states.
func (t *ErrorTracker) Analyze() {
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
			initialState := newErrorState()
			t.analyzeStatements(fn.Body.List, initialState)
			t.fnStates[fn] = initialState.clone()
		}
	}
}

// SetCurrentFunc sets the active function declaration and enclosing file.
func (t *ErrorTracker) SetCurrentFunc(file *ast.File, fn *ast.FuncDecl) {
	t.currentFile = file
	t.currentFn = fn
}

func (t *ErrorTracker) analyzeStatements(stmts []ast.Stmt, state *errorState) {
	for _, stmt := range stmts {
		ast.Inspect(stmt, func(n ast.Node) bool {
			if n != nil {
				t.nodeStates[n] = state.clone()
			}
			return true
		})

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

func (t *ErrorTracker) handleAssign(stmt *ast.AssignStmt, state *errorState) {
	if stmt.Tok == token.DEFINE {
		for i, lhs := range stmt.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			var rhs ast.Expr
			if i < len(stmt.Rhs) {
				rhs = stmt.Rhs[i]
			} else if len(stmt.Rhs) == 1 {
				rhs = stmt.Rhs[0]
			} else {
				continue
			}
			k := makeDefVarKey(t.pass, ident)
			v := evalExprOrigin(t.pass, t.currentFile, t.currentFn, rhs, state)
			state.set(k, v)
		}
	} else if stmt.Tok == token.ASSIGN {
		for i, lhs := range stmt.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok {
				continue
			}
			var rhs ast.Expr
			if i < len(stmt.Rhs) {
				rhs = stmt.Rhs[i]
			} else if len(stmt.Rhs) == 1 {
				rhs = stmt.Rhs[0]
			} else {
				continue
			}
			k := makeVarKey(t.pass, t.currentFile, t.currentFn, ident)
			v := evalExprOrigin(t.pass, t.currentFile, t.currentFn, rhs, state)
			state.set(k, v)
		}
	}
}

func (t *ErrorTracker) handleDecl(stmt *ast.DeclStmt, state *errorState) {
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
				v := evalExprOrigin(t.pass, t.currentFile, t.currentFn, valSpec.Values[i], state)
				state.set(k, v)
			} else {
				state.set(k, errorValue{kind: errorKindClean})
			}
		}
	}
}

// GetErrorStateAt returns the reaching error value for expr at the given AST node.
func (t *ErrorTracker) GetErrorStateAt(fn *ast.FuncDecl, expr ast.Expr, at ast.Node) errorValue {
	if t == nil || expr == nil {
		return errorValue{kind: errorKindClean}
	}
	activeFn := fn
	if activeFn == nil {
		activeFn = t.currentFn
	}
	var state *errorState
	if at != nil {
		state = t.nodeStates[at]
	}
	if state == nil && activeFn != nil {
		state = t.fnStates[activeFn]
	}
	if state == nil {
		state = newErrorState()
	}
	return evalExprOrigin(t.pass, t.currentFile, activeFn, expr, state)
}

