// Package a08_tx_io traverses interprocedural calls inside transaction bodies to detect indirect blocking I/O.
package a08_tx_io

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// CheckTxNode inspects a node inside a transaction body, directly or by walking helper function definitions.
func CheckTxNode(pass *analysis.Pass, n ast.Node, funcDecls map[string]*ast.FuncDecl, visited map[string]bool, dm *directives.DirectiveMap) {
	// 1. Direct blocking I/O on node
	if op := CheckBlockingIO(n, pass); op != "" {
		if !dm.IsIgnored(pass.Fset, n.Pos(), RuleCode) {
			pass.Reportf(n.Pos(), "[%s] blocking external I/O (%s) detected inside database transaction; holding open transactions causes connection pool starvation and lock cascade", RuleCode, op)
		}
		return
	}

	// 2. Interprocedural call traversal to local functions
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return
	}

	funcName := getCallMethodName(call.Fun)
	if funcName == "" || visited[funcName] {
		return
	}

	targetFn, found := funcDecls[funcName]
	if !found || targetFn.Body == nil {
		return
	}

	// Mark visited to avoid recursion cycle
	visited[funcName] = true
	defer func() { visited[funcName] = false }()

	ast.Inspect(targetFn.Body, func(innerNode ast.Node) bool {
		if innerNode == nil {
			return true
		}
		if op := CheckBlockingIO(innerNode, pass); op != "" {
			if !dm.IsIgnored(pass.Fset, call.Pos(), RuleCode) && !dm.IsIgnored(pass.Fset, innerNode.Pos(), RuleCode) {
				pass.Reportf(call.Pos(), "[%s] blocking external I/O (%s) detected inside database transaction; holding open transactions causes connection pool starvation and lock cascade", RuleCode, op)
			}
		}
		return true
	})
}
