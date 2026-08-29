// Package a08_tx_io enforces that open database transactions do not enclose
// blocking external I/O operations (HTTP, network, disk, sleep, command execution).
package a08_tx_io

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A08"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A08.
var Analyzer = &analysis.Analyzer{
	Name: "a08",
	Doc:  "Prohibits blocking external I/O (network, HTTP, disk, sleep, exec) inside database transactions",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	for _, file := range pass.Files {
		pos := pass.Fset.Position(file.Package)
		if strings.HasSuffix(pos.Filename, "_test.go") {
			continue
		}

		funcDecls := make(map[string]*ast.FuncDecl)
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				funcDecls[fn.Name.Name] = fn
			}
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			inspectFunctionTransactions(pass, fn.Body, funcDecls, dm)
		}
	}

	return nil, nil
}

func inspectFunctionTransactions(pass *analysis.Pass, body *ast.BlockStmt, funcDecls map[string]*ast.FuncDecl, dm *directives.DirectiveMap) {
	visited := make(map[string]bool)

	// 1. Check closure-based transactions (BeginFunc, ExecuteTx, WithTx)
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		closure := ExtractTxClosure(call)
		if closure != nil && closure.Body != nil {
			ast.Inspect(closure.Body, func(innerNode ast.Node) bool {
				CheckTxNode(pass, innerNode, funcDecls, visited, dm)
				return true
			})
		}
		return true
	})

	// 2. Check explicit transaction blocks (pool.Begin ... tx.Commit)
	InspectExplicitTxRanges(body, func(stmt ast.Stmt) {
		ast.Inspect(stmt, func(n ast.Node) bool {
			CheckTxNode(pass, n, funcDecls, visited, dm)
			return true
		})
	})
}
