// Package a02_unclosed_rows enforces that every pgx.Rows produced by Query()
// is safely closed via defer rows.Close() or consumed by an auto-closing helper.
package a02_unclosed_rows

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A02"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A02.
var Analyzer = &analysis.Analyzer{
	Name: "a02",
	Doc:  "Enforces defer rows.Close() or auto-closing helper for all Query() calls",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A02.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for unclosed rows and prohibited ownership transfers.
// Can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap) []Issue {
	if file == nil {
		return nil
	}
	if fset == nil && pass != nil {
		fset = pass.Fset
	}

	var issues []Issue
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		checkBlock(fset, fn.Body, dm, &issues)
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

func checkBlock(fset *token.FileSet, body *ast.BlockStmt, dm *directives.DirectiveMap, issues *[]Issue) {
	ast.Inspect(body, func(n ast.Node) bool {
		// If nested closure, inspect it separately
		if lit, ok := n.(*ast.FuncLit); ok && lit.Body != nil {
			checkBlock(fset, lit.Body, dm, issues)
			return false
		}

		// Pattern 1: Direct return of Query(...) call
		if ret, ok := n.(*ast.ReturnStmt); ok {
			for _, expr := range ret.Results {
				if IsQueryCall(expr) {
					if fset == nil || dm == nil || !dm.IsIgnored(fset, expr.Pos(), RuleCode) {
						*issues = append(*issues, Issue{
							Pos:     expr.Pos(),
							Message: "returning rows transfers resource ownership; consume and close rows within this function",
						})
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
			if fset != nil && dm != nil && dm.IsIgnored(fset, rhs.Pos(), RuleCode) {
				continue
			}

			// Check if rows returned to caller (prohibited ownership transfer)
			if IsReturned(body, rowsVar) {
				*issues = append(*issues, Issue{
					Pos:     rhs.Pos(),
					Message: fmt.Sprintf("returning rows variable %q transfers resource ownership; consume and close rows within this function", rowsVar),
				})
				continue
			}

			// Check if safely deferred or consumed by auto-closing helper
			if !IsAssignSafelyClosed(body, assign, rowsVar) {
				*issues = append(*issues, Issue{
					Pos:     rhs.Pos(),
					Message: fmt.Sprintf("missing defer %s.Close() for Query() call; unclosed rows leak database connections and cause pool starvation", rowsVar),
				})
			}
		}

		return true
	})
}
