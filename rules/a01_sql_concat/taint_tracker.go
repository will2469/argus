// Package a01_sql_concat tracks taint propagation and detects unsafe SQL string assembly.
package a01_sql_concat

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type TaintTracker struct {
	pass       *analysis.Pass
	files      []*ast.File
	currentFn  *ast.FuncDecl
	tainted    map[types.Object]struct{}
	sources    map[types.Object]struct{}
	astTainted map[*ast.FuncDecl]map[string]struct{}
	astSources map[*ast.FuncDecl]map[string]struct{}
}

func NewTaintTracker(pass *analysis.Pass, files ...*ast.File) *TaintTracker {
	var fList []*ast.File
	if pass != nil {
		fList = pass.Files
	} else {
		fList = files
	}
	return &TaintTracker{
		pass:       pass,
		files:      fList,
		tainted:    make(map[types.Object]struct{}),
		sources:    make(map[types.Object]struct{}),
		astTainted: make(map[*ast.FuncDecl]map[string]struct{}),
		astSources: make(map[*ast.FuncDecl]map[string]struct{}),
	}
}

// SetCurrentFunc sets the active function context for AST-based taint analysis.
func (t *TaintTracker) SetCurrentFunc(fn *ast.FuncDecl) {
	t.currentFn = fn
}

func (t *TaintTracker) markTaintedName(name string) {
	if t.currentFn == nil {
		return
	}
	if t.astTainted[t.currentFn] == nil {
		t.astTainted[t.currentFn] = make(map[string]struct{})
	}
	t.astTainted[t.currentFn][name] = struct{}{}
}

func (t *TaintTracker) deleteTaintedName(name string) {
	if t.currentFn != nil && t.astTainted[t.currentFn] != nil {
		delete(t.astTainted[t.currentFn], name)
	}
}

func (t *TaintTracker) Analyze() {
	for _, file := range t.files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			t.currentFn = fn

			// Phase 1: Mark taint sources (function params like id, query, req DTOs)
			if fn.Type.Params != nil {
				t.markSources(fn)
			}

			// Phase 2: Propagate taint through assignments and builder calls
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.AssignStmt:
					t.propagateAssign(x)
				case *ast.ExprStmt:
					if call, ok := x.X.(*ast.CallExpr); ok {
						PropagateBuilderCall(t, call)
					}
				}
				return true
			})
		}
	}
	t.currentFn = nil
}

func (t *TaintTracker) markSources(fn *ast.FuncDecl) {
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if t.pass != nil && t.pass.TypesInfo != nil {
				obj := t.pass.TypesInfo.Defs[name]
				if obj != nil && isTaintSource(name.Name, obj.Type()) {
					t.sources[obj] = struct{}{}
					t.tainted[obj] = struct{}{}
				}
			} else {
				typeStr := astExprToString(field.Type)
				if isTaintSourceAST(name.Name, typeStr) {
					if t.astSources[fn] == nil {
						t.astSources[fn] = make(map[string]struct{})
					}
					t.astSources[fn][name.Name] = struct{}{}
					t.markTaintedName(name.Name)
				}
			}
		}
	}
}

func isTaintSourceAST(name string, typeStr string) bool {
	switch strings.ToLower(name) {
	case "id", "nik", "email", "userid", "user_id", "param", "params", "query", "q",
		"search", "filter", "sort", "order", "orderby", "order_by", "table", "column", "rawsql", "sql":
		return true
	}
	if typeStr != "" {
		if strings.HasSuffix(typeStr, "Request") || strings.HasSuffix(typeStr, "DTO") ||
			strings.HasSuffix(typeStr, "Input") || strings.HasSuffix(typeStr, "Params") ||
			strings.HasSuffix(typeStr, "Filter") || strings.Contains(typeStr, "http.Request") {
			return true
		}
	}
	return false
}

func astExprToString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return "*" + astExprToString(x.X)
	case *ast.SelectorExpr:
		return astExprToString(x.X) + "." + x.Sel.Name
	default:
		return ""
	}
}

func isTaintSource(name string, typ types.Type) bool {
	switch strings.ToLower(name) {
	case "id", "nik", "email", "userid", "user_id", "param", "params", "query", "q",
		"search", "filter", "sort", "order", "orderby", "order_by", "table", "column", "rawsql", "sql":
		return true
	}
	if typ != nil {
		s := typ.String()
		if strings.HasSuffix(s, "Request") || strings.HasSuffix(s, "DTO") ||
			strings.HasSuffix(s, "Input") || strings.HasSuffix(s, "Params") ||
			strings.HasSuffix(s, "Filter") || strings.Contains(s, "http.Request") {
			return true
		}
	}
	return false
}

func (t *TaintTracker) propagateAssign(stmt *ast.AssignStmt) {
	tainted := false
	for _, rhs := range stmt.Rhs {
		if t.IsTaintedExpr(rhs) {
			tainted = true
			break
		}
	}
	for _, lhs := range stmt.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok {
			continue
		}
		if t.pass != nil && t.pass.TypesInfo != nil {
			obj := t.pass.TypesInfo.Defs[ident]
			if obj == nil {
				obj = t.pass.TypesInfo.Uses[ident]
			}
			if obj != nil {
				if tainted {
					t.tainted[obj] = struct{}{}
				} else if stmt.Tok == token.ASSIGN {
					delete(t.tainted, obj)
				}
			}
		} else if t.currentFn != nil {
			if tainted {
				t.markTaintedName(ident.Name)
			} else if stmt.Tok == token.ASSIGN {
				t.deleteTaintedName(ident.Name)
			}
		}
	}
}

// IsTaintedExpr checks if an expression carries tainted data or dynamic formatting.
func (t *TaintTracker) IsTaintedExpr(e ast.Expr) bool {
	if e == nil {
		return false
	}
	if IsSanitized(e) {
		return false
	}

	switch x := e.(type) {
	case *ast.BasicLit:
		return false // String constants are safe compile-time literals
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			_, xLit := x.X.(*ast.BasicLit)
			_, yLit := x.Y.(*ast.BasicLit)
			if xLit && yLit {
				return false
			}
			if (xLit && IsSanitized(x.Y)) || (yLit && IsSanitized(x.X)) {
				return false
			}
			return t.IsTaintedExpr(x.X) || t.IsTaintedExpr(x.Y)
		}
	case *ast.Ident:
		if t.pass != nil && t.pass.TypesInfo != nil {
			if obj := t.pass.TypesInfo.Uses[x]; obj != nil {
				if _, ok := t.tainted[obj]; ok {
					return true
				}
				if _, ok := t.sources[obj]; ok {
					return true
				}
			}
		} else if t.currentFn != nil {
			if taintedVars, ok := t.astTainted[t.currentFn]; ok {
				if _, ok := taintedVars[x.Name]; ok {
					return true
				}
			}
			if sourceVars, ok := t.astSources[t.currentFn]; ok {
				if _, ok := sourceVars[x.Name]; ok {
					return true
				}
			}
		}
	case *ast.SelectorExpr:
		return t.IsTaintedExpr(x.X)
	case *ast.CallExpr:
		if IsFormattingCall(x, t) {
			return true
		}
		if IsBuilderString(x) {
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				return t.IsTaintedExpr(sel.X)
			}
		}
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			if t.IsTaintedExpr(sel.X) {
				return true
			}
		}
		for _, arg := range x.Args {
			if t.IsTaintedExpr(arg) {
				return true
			}
		}
	}
	return false
}
