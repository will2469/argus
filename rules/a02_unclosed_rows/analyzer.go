// Package a02_unclosed_rows enforces that every pgx.Rows produced by Query()
// is safely closed via defer rows.Close() or consumed by an auto-closing helper.
package a02_unclosed_rows

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A02"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A02.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a02_missing_defer_close",
	Doc:  "Enforces defer rows.Close() or auto-closing helper for all Query() calls",
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
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			checkBlock(pass, fn.Body, dm)
		}
	}

	return nil, nil
}

func checkBlock(pass *analysis.Pass, body *ast.BlockStmt, dm *directives.DirectiveMap) {
	ast.Inspect(body, func(n ast.Node) bool {
		// If nested closure, inspect it separately
		if lit, ok := n.(*ast.FuncLit); ok && lit.Body != nil {
			checkBlock(pass, lit.Body, dm)
			return false
		}

		// Pattern 1: Direct return of Query(...) call
		if ret, ok := n.(*ast.ReturnStmt); ok {
			for _, expr := range ret.Results {
				if IsQueryCall(expr) {
					if !dm.IsIgnored(pass.Fset, expr.Pos(), RuleCode) {
						pass.Reportf(expr.Pos(), "[%s] returning rows transfers resource ownership; consume and close rows within this function", RuleCode)
					}
				}
			}
		}

		// Pattern 2: Assignment rows, err := db.Query(...)
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for _, rhs := range assign.Rhs {
			if !IsQueryCall(rhs) {
				continue
			}

			if len(assign.Lhs) == 0 {
				continue
			}

			ident, ok := assign.Lhs[0].(*ast.Ident)
			if !ok || ident.Name == "_" {
				continue
			}

			rowsVar := ident.Name
			if dm.IsIgnored(pass.Fset, rhs.Pos(), RuleCode) {
				continue
			}

			// Check if rows returned to caller (prohibited ownership transfer)
			if IsReturned(body, rowsVar) {
				pass.Reportf(rhs.Pos(), "[%s] returning rows variable %q transfers resource ownership; consume and close rows within this function", RuleCode, rowsVar)
				continue
			}

			// Check if safely deferred or consumed by auto-closing helper
			if !IsSafelyClosedOrConsumed(body, rowsVar) {
				pass.Reportf(rhs.Pos(), "[%s] missing defer %s.Close() for Query() call; unclosed rows leak database connections and cause pool starvation", RuleCode, rowsVar)
			}
		}

		return true
	})
}
