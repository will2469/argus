// Package a20_param_limit enforces PostgreSQL 65,535 wire protocol parameter bounds
// on dynamic multi-row and IN clause statements, promoting pgx.CopyFrom and ANY($1).
package a20_param_limit

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A20.
const RuleCode = "ARGUS-A20"

// Analyzer defines the analysis.Analyzer for ARGUS-A20.
var Analyzer = &analysis.Analyzer{
	Name: "a20",
	Doc:  "Enforce PostgreSQL 65,535 bind parameter limit in dynamic multi-row statements; recommend pgx.CopyFrom (CWE-400)",
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

		InspectFile(file, pass.Fset, dm, func(pos token.Pos, format string, args ...any) {
			pass.Reportf(pos, format, args...)
		})
	}

	return nil, nil
}

// InspectFile walks an AST file and reports violations of ARGUS-A20.
func InspectFile(file *ast.File, fset *token.FileSet, dm *directives.DirectiveMap, report func(pos token.Pos, format string, args ...any)) {
	var currentFunc ast.Node

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		switch n.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			currentFunc = n
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		methodName := callsite.GetCallMethodName(call.Fun)
		if methodName == "CopyFrom" {
			// Binary COPY protocol is inherently immune to parameter limits
			return true
		}

		if !callsite.IsDBQueryMethod(methodName) {
			return true
		}

		query, _ := callsite.ExtractQueryString(call)

		if kind, msg := EvaluateDynamicBatch(currentFunc, call, query); kind != BatchKindNone {
			if dm != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".PARAM-LIMIT")) {
				return true
			}
			report(call.Pos(), "[%s] %s", RuleCode, msg)
		}

		return true
	})
}
