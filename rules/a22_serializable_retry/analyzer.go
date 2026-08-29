// Package a22_serializable_retry enforces automatic retry loops on Serializable and RepeatableRead
// transactions to prevent unhandled 500 serialization abort errors (SQLSTATE 40001, 40P01).
package a22_serializable_retry

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A22.
const RuleCode = "ARGUS-A22"

// Analyzer defines the analysis.Analyzer for ARGUS-A22.
var Analyzer = &analysis.Analyzer{
	Name: "a22",
	Doc:  "Enforce automatic retry loops on Serializable and RepeatableRead transactions catching SQLSTATE 40001 and 40P01 (CWE-362)",
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

// InspectFile walks an AST file and reports violations of ARGUS-A22.
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
		if methodName != "BeginTx" {
			return true
		}

		// Check if any argument specifies Serializable or RepeatableRead
		isStrict := false
		for _, arg := range call.Args {
			if IsStrictIsolationLevel(arg) {
				isStrict = true
				break
			}
		}

		if !isStrict {
			return true
		}

		// Verify if call is enclosed within a retry loop or retry helper
		if HasRetryEnclosure(currentFunc, call) {
			return true
		}

		if dm != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".SERIALIZABLE-RETRY")) {
			return true
		}

		report(call.Pos(), "[%s] single-shot '%s' transaction with strict isolation without automated retry loop; risk of unhandled serialization abort (SQLSTATE 40001, CWE-362)", RuleCode, methodName)
		return true
	})
}
