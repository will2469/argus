package a17_nplusone

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// HelperQueryDetector caches and detects functions that execute database queries.
type HelperQueryDetector struct {
	funcHasQuery map[string]bool
	funcDecls    map[string]*ast.FuncDecl
}

// NewHelperQueryDetector builds a local call graph index for the current file.
func NewHelperQueryDetector(pass *analysis.Pass, file *ast.File) *HelperQueryDetector {
	detector := &HelperQueryDetector{
		funcHasQuery: make(map[string]bool),
		funcDecls:    make(map[string]*ast.FuncDecl),
	}

	// 1. Index all functions in the file
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Body == nil {
			continue
		}
		key := getFuncKey(fn)
		detector.funcDecls[key] = fn
	}

	// 2. Mark functions that execute DB queries directly
	for key, fn := range detector.funcDecls {
		if containsDirectDBQuery(pass, fn.Body) {
			detector.funcHasQuery[key] = true
		}
	}

	// 3. Fixed-point propagation: iteratively propagate query execution through the call graph
	// until a fixed point is reached (no new helper functions marked across arbitrary call depth).
	changed := true
	for changed {
		changed = false
		for key, fn := range detector.funcDecls {
			if detector.funcHasQuery[key] {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if detector.funcHasQuery[key] {
					return false
				}
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				calledKey := getCallKey(call)
				if detector.funcHasQuery[calledKey] {
					detector.funcHasQuery[key] = true
					changed = true
					return false
				}
				return true
			})
		}
	}

	return detector
}

// CheckHelperCall checks if a call expression invokes a helper function that executes a DB query.
func (d *HelperQueryDetector) CheckHelperCall(pass *analysis.Pass, call *ast.CallExpr) (bool, string) {
	if d == nil || call == nil {
		return false, ""
	}

	key := getCallKey(call)
	if d.funcHasQuery[key] && isQueryHelperName(key) {
		return true, key
	}

	return false, ""
}

func isQueryHelperName(name string) bool {
	lower := strings.ToLower(name)
	prefixes := []string{"get", "fetch", "find", "load", "read", "select", "lookup"}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return strings.Contains(lower, "query")
}

func containsDirectDBQuery(pass *analysis.Pass, body *ast.BlockStmt) bool {
	hasQuery := false
	ast.Inspect(body, func(n ast.Node) bool {
		if hasQuery {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if IsDBQueryCall(pass, call) {
			hasQuery = true
			return false
		}
		return true
	})
	return hasQuery
}

func getFuncKey(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		return fn.Name.Name
	}
	return fn.Name.Name
}

func getCallKey(call *ast.CallExpr) string {
	switch e := call.Fun.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}
