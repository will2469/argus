// Package a25_expensive_cpu enforces that active database transactions do not enclose
// CPU-expensive operations (password hashing, key derivation, asymmetric keygen, subprocess exec)
// to prevent connection pool exhaustion and lock duration inflation (CWE-400, CWE-662).
package a25_expensive_cpu

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A25.
const RuleCode = "ARGUS-A25"

// Analyzer defines the analysis.Analyzer for ARGUS-A25.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a25_expensive_cpu_in_transaction",
	Doc:  "Prohibit CPU-expensive computations (bcrypt, argon2, RSA keygen, PDF rendering) inside active database transactions (CWE-400, CWE-662)",
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

	// 1. Check closure-based transactions (BeginFunc, ExecuteTx, ExecuteLockedTx, WithTx)
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		closure := ExtractTxClosure(call)
		if closure != nil && closure.Body != nil {
			ast.Inspect(closure.Body, func(innerNode ast.Node) bool {
				CheckTxNode(innerNode, funcDecls, visited, pass.Fset, dm, func(pos token.Pos, reason string) {
					pass.Reportf(pos, "[%s] %s", RuleCode, reason)
				})
				return true
			})
		}
		return true
	})

	// 2. Check explicit transaction blocks (pool.Begin ... tx.Commit)
	InspectExplicitTxRanges(body, func(stmt ast.Stmt) {
		ast.Inspect(stmt, func(n ast.Node) bool {
			CheckTxNode(n, funcDecls, visited, pass.Fset, dm, func(pos token.Pos, reason string) {
				pass.Reportf(pos, "[%s] %s", RuleCode, reason)
			})
			return true
		})
	})
}
