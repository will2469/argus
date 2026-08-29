// Package a01_sql_concat prohibits dynamic SQL concatenation and string formatting
// in favor of compile-time string constants with parameterized placeholders ($1, $2, ...).
package a01_sql_concat

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A01"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A01.
var Analyzer = &analysis.Analyzer{
	Name: "a01",
	Doc:  "Prohibits dynamic SQL concatenation and formatting without parameterization or identifier sanitization",
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

	tracker := NewTaintTracker(pass)
	tracker.Analyze()

	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
				return true
			}

			sqlArg := extractSQLArgument(call)
			if sqlArg == nil {
				return true
			}

			if dm.IsIgnored(pass.Fset, sqlArg.Pos(), RuleCode) {
				return true
			}

			if isUnsafeSQL(sqlArg, tracker) {
				pass.Reportf(sqlArg.Pos(), "[%s] unsafe SQL concatenation or formatting; use parameterized placeholders ($1, $2, ...) or SanitizeIdentifier", RuleCode)
			}
			return true
		})
	}

	return nil, nil
}

func extractSQLArgument(call *ast.CallExpr) ast.Expr {
	if len(call.Args) == 0 {
		return nil
	}
	// Common signature: (ctx, sql, args...) or (sql, args...)
	if len(call.Args) >= 2 {
		return call.Args[1]
	}
	return call.Args[0]
}

func isUnsafeSQL(e ast.Expr, tracker *TaintTracker) bool {
	if e == nil {
		return false
	}
	if IsSanitized(e) {
		return false
	}

	switch x := e.(type) {
	case *ast.BasicLit:
		return false // Pure string literal is safe
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			_, xLit := x.X.(*ast.BasicLit)
			_, yLit := x.Y.(*ast.BasicLit)
			if xLit && yLit {
				return false
			}
			if (xLit && IsSanitized(x.Y)) || (yLit && IsSanitized(x.X)) {
				return false
			}
			return true
		}
	case *ast.CallExpr:
		if IsFormattingCall(x, tracker) {
			return true
		}
		if IsBuilderString(x) {
			return tracker.IsTaintedExpr(x)
		}
	case *ast.Ident:
		return tracker.IsTaintedExpr(x)
	}

	return false
}
