// Package a06_runtime_ddl prohibits executing DDL statements in Go application runtime code.
package a06_runtime_ddl

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A06"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A06.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a06_runtime_ddl",
	Doc:  "Prohibits execution of DDL statements (CREATE, ALTER, DROP, TRUNCATE, GRANT, REVOKE) in runtime Go application code",
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

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
					return true
				}

				if dm.IsIgnored(pass.Fset, call.Pos(), RuleCode) {
					return true
				}

				// 1. Static AST detection on literal query strings
				if query, found := callsite.ExtractQueryString(call); found && strings.TrimSpace(query) != "" {
					if ddlOp := DetectDDLFromAST(query); ddlOp != "" {
						pass.Reportf(call.Pos(), "[%s] runtime database query contains forbidden DDL statement (%s); DDL is restricted to migrations", RuleCode, ddlOp)
						return true
					}
				}

				// 2. Dynamic DDL detection on all arguments of the call
				for _, arg := range call.Args {
					if ddlOp := DetectDynamicDDL(arg, fn.Body); ddlOp != "" {
						pass.Reportf(call.Pos(), "[%s] runtime database query contains forbidden DDL statement (%s); DDL is restricted to migrations", RuleCode, ddlOp)
						return true
					}
				}

				return true
			})
		}
	}

	return nil, nil
}
