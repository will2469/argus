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

// Issue describes a detected violation of ARGUS-A01.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for dynamic SQL concatenation or formatting.
// It can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap) []Issue {
	tracker := NewTaintTracker(pass, file)
	tracker.Analyze()
	return InspectFileWithTracker(pass, fset, file, dm, tracker)
}

// InspectFileWithTracker inspects an AST file using a pre-initialized TaintTracker.
func InspectFileWithTracker(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap, tracker *TaintTracker) []Issue {
	var issues []Issue
	if file == nil {
		return nil
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if tracker != nil {
			tracker.SetCurrentFunc(fn)
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

			sqlArg := extractSQLArgument(call)
			if sqlArg == nil {
				return true
			}

			if dm != nil && fset != nil && dm.IsIgnored(fset, sqlArg.Pos(), RuleCode) {
				return true
			}

			if isUnsafeSQL(sqlArg, tracker) {
				issues = append(issues, Issue{
					Pos:     sqlArg.Pos(),
					Message: "unsafe SQL concatenation or formatting; use parameterized placeholders ($1, $2, ...) or SanitizeIdentifier",
				})
			}
			return true
		})
	}
	if tracker != nil {
		tracker.SetCurrentFunc(nil)
	}

	return issues
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	tracker := NewTaintTracker(pass, pass.Files...)
	tracker.Analyze()

	for _, file := range pass.Files {
		issues := InspectFileWithTracker(pass, pass.Fset, file, dm, tracker)
		for _, issue := range issues {
			pass.Reportf(issue.Pos, "[%s] %s", RuleCode, issue.Message)
		}
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
			return tracker != nil && tracker.IsTaintedExpr(x)
		}
	case *ast.Ident:
		return tracker != nil && tracker.IsTaintedExpr(x)
	}

	return false
}
