// Package a01_sql_concat tracks taint propagation and detects unsafe SQL string assembly.
package a01_sql_concat

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// TaintTracker tracks flow-sensitive taint across control-flow paths.
type TaintTracker struct {
	pass          *analysis.Pass
	files         []*ast.File
	currentFn     *ast.FuncDecl
	nodeStates    map[ast.Node]*taintState
	sources       map[types.Object]struct{}
	astSources    map[*ast.FuncDecl]map[string]struct{}
	customSources map[string]struct{}
}

// NewTaintTracker initializes a flow-sensitive TaintTracker.
func NewTaintTracker(pass *analysis.Pass, files ...*ast.File) *TaintTracker {
	var fList []*ast.File
	if pass != nil {
		fList = pass.Files
	} else {
		fList = files
	}
	return &TaintTracker{
		pass:          pass,
		files:         fList,
		nodeStates:    make(map[ast.Node]*taintState),
		sources:       make(map[types.Object]struct{}),
		astSources:    make(map[*ast.FuncDecl]map[string]struct{}),
		customSources: make(map[string]struct{}),
	}
}

// SetCustomTaintSources registers domain-specific parameter names (from .argus.yaml) as untrusted sources.
func (t *TaintTracker) SetCustomTaintSources(sources []string) {
	if len(sources) == 0 {
		return
	}
	if t.customSources == nil {
		t.customSources = make(map[string]struct{})
	}
	for _, s := range sources {
		trimmed := strings.ToLower(strings.TrimSpace(s))
		if trimmed != "" {
			t.customSources[trimmed] = struct{}{}
		}
	}
}

// SetCurrentFunc sets the active function context.
func (t *TaintTracker) SetCurrentFunc(fn *ast.FuncDecl) {
	t.currentFn = fn
}

// Analyze performs flow-sensitive taint analysis across all functions.
func (t *TaintTracker) Analyze() {
	for _, file := range t.files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			t.currentFn = fn
			state := newTaintState()

			// Phase 1: Seed initial taint from function parameters
			if fn.Type.Params != nil {
				t.markSources(fn, state)
			}

			// Phase 2: Flow-sensitive statement traversal
			t.analyzeStatements(fn.Body.List, state)
		}
	}
	t.currentFn = nil
}

func (t *TaintTracker) markSources(fn *ast.FuncDecl, state *taintState) {
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if t.pass != nil && t.pass.TypesInfo != nil {
				obj := t.pass.TypesInfo.Defs[name]
				if obj != nil && isTaintSource(name.Name, obj.Type(), t.customSources) {
					t.sources[obj] = struct{}{}
					state.markTainted(name, t.pass)
				}
			} else {
				typeStr := astExprToString(field.Type)
				if isTaintSourceAST(name.Name, typeStr, t.customSources) {
					if t.astSources[fn] == nil {
						t.astSources[fn] = make(map[string]struct{})
					}
					t.astSources[fn][name.Name] = struct{}{}
					state.markTainted(name, nil)
				}
			}
		}
	}
}

func (t *TaintTracker) isExprTaintedInState(e ast.Expr, state *taintState) bool {
	if e == nil || state == nil {
		return false
	}
	if IsSanitized(e, t.pass) {
		return false
	}

	switch x := e.(type) {
	case *ast.BasicLit:
		return false
	case *ast.ParenExpr:
		return t.isExprTaintedInState(x.X, state)
	case *ast.UnaryExpr:
		return t.isExprTaintedInState(x.X, state)
	case *ast.StarExpr:
		return t.isExprTaintedInState(x.X, state)
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return !IsSafeConcat(x, t.pass)
		}
	case *ast.Ident:
		return state.isIdentTainted(x, t.pass)
	case *ast.SelectorExpr:
		return t.isExprTaintedInState(x.X, state)
	case *ast.CallExpr:
		if IsFormattingCall(x, t, t.pass) {
			return true
		}
		if IsBuilderString(x) {
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				return t.isExprTaintedInState(sel.X, state)
			}
		}
		for _, arg := range x.Args {
			if t.isExprTaintedInState(arg, state) {
				return true
			}
		}
	}
	return false
}

// IsTaintedAt checks if expr is tainted at the given program point.
func (t *TaintTracker) IsTaintedAt(expr ast.Expr, at ast.Node) bool {
	if t == nil || expr == nil {
		return false
	}
	var state *taintState
	if at != nil {
		state = t.nodeStates[at]
	}
	if state == nil {
		return t.IsTaintedExpr(expr)
	}
	return t.isExprTaintedInState(expr, state)
}

// IsTaintedExpr checks if an expression is tainted under any recorded state or source.
func (t *TaintTracker) IsTaintedExpr(e ast.Expr) bool {
	if t == nil || e == nil {
		return false
	}
	if state, ok := t.nodeStates[e]; ok {
		return t.isExprTaintedInState(e, state)
	}
	if id, ok := e.(*ast.Ident); ok {
		if t.pass != nil && t.pass.TypesInfo != nil {
			if obj := t.pass.TypesInfo.Uses[id]; obj != nil {
				if _, ok := t.sources[obj]; ok {
					return true
				}
			}
		}
		if t.currentFn != nil {
			if srcMap, ok := t.astSources[t.currentFn]; ok {
				if _, ok := srcMap[id.Name]; ok {
					return true
				}
			}
		}
	}
	return false
}
