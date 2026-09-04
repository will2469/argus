// Package a08_tx_io enforces that open database transactions do not enclose
// blocking external I/O operations (HTTP, network, disk, sleep, command execution).
package a08_tx_io

import (
	"go/ast"
	"go/token"
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

// Issue describes a detected violation of ARGUS-A08.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for blocking I/O within database transactions.
// Can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap) []Issue {
	if file == nil {
		return nil
	}
	if fset == nil && pass != nil {
		fset = pass.Fset
	}

	pos := fset.Position(file.Package)
	if strings.HasSuffix(pos.Filename, "_test.go") {
		return nil
	}

	var funcDecls []*ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			funcDecls = append(funcDecls, fn)
		}
	}

	var issues []Issue
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		inspectFunctionTransactions(pass, fset, fn, file, funcDecls, dm, &issues)
	}

	return issues
}

func inspectFunctionTransactions(pass *analysis.Pass, fset *token.FileSet, fn *ast.FuncDecl, file *ast.File, funcDecls []*ast.FuncDecl, dm *directives.DirectiveMap, issues *[]Issue) {
	if fn == nil || fn.Body == nil {
		return
	}
	visited := make(map[*ast.FuncDecl]bool)

	// 1. Check closure-based transactions (BeginFunc, ExecuteTx, WithTx)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		closure := ExtractTxClosure(pass, call, fn, file)
		if closure != nil && closure.Body != nil {
			ast.Inspect(closure.Body, func(innerNode ast.Node) bool {
				CheckTxNode(pass, fset, innerNode, fn, file, funcDecls, visited, dm, issues)
				return true
			})
		}
		return true
	})

	// 2. Flow-sensitive path analysis for explicit transaction lifecycles (pool.Begin ... tx.Commit)
	InspectExplicitTxFlow(pass, fset, fn, file, funcDecls, visited, dm, issues)
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	for _, file := range pass.Files {
		issues := InspectFile(pass, pass.Fset, file, dm)
		for _, iss := range issues {
			pass.Reportf(iss.Pos, "[%s] %s", RuleCode, iss.Message)
		}
	}

	return nil, nil
}
