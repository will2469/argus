// Package a07_error_leak prohibits exposing raw database driver errors,
// pgconn.PgError fields (Detail, Hint, Where), or raw err.Error() strings into API responses.
package a07_error_leak

import (
	"go/ast"
	"go/token"
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

// Issue describes a detected violation of ARGUS-A07.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for database error leakage into API responses.
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

	var issues []Issue
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SelectorExpr:
				CheckPgErrorSensitiveFields(pass, fset, node, dm, &issues)
			case *ast.CallExpr:
				CheckErrorFactoryCall(pass, fset, node, fn, dm, &issues)
				sink := InspectResponseSink(pass, node, fn)
				if sink.IsSink {
					for _, arg := range sink.Args {
						CheckLeakedErrorArg(pass, fset, arg, node.Pos(), fn, dm, &issues)
					}
				}
			}
			return true
		})
	}

	return issues
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
