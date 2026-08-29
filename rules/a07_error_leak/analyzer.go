// Package a07_error_leak prohibits exposing raw database driver errors,
// pgconn.PgError fields (Detail, Hint, Where), or raw err.Error() strings into API responses.
package a07_error_leak

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A07"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A07.
var Analyzer = &analysis.Analyzer{
	Name: "a07",
	Doc:  "Detects exposure of raw database errors and pgconn.PgError Detail/Hint/Where into API responses",
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
				switch node := n.(type) {
				case *ast.SelectorExpr:
					CheckPgErrorSensitiveFields(pass, node, dm)
				case *ast.CallExpr:
					sink := InspectResponseSink(node)
					if sink.IsSink {
						for _, arg := range sink.Args {
							CheckLeakedErrorArg(pass, arg, node.Pos(), fn.Body, dm)
						}
					}
				}
				return true
			})
		}
	}

	return nil, nil
}
