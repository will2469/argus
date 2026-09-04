// Package a09_advisory_lock ensures safe PostgreSQL advisory lock usage by prohibiting
// session-level locks on connection pools and forbidding hardcoded integer magic numbers.
package a09_advisory_lock

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A09"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A09.
var Analyzer = &analysis.Analyzer{
	Name: "a09",
	Doc:  "Enforces transaction-scoped advisory locks and namespaced identifiers",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A09.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for unsafe advisory lock usages.
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
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			checkDBCallAdvisoryLock(pass, fset, call, fn.Body, dm, &issues)
			CheckAdvisoryHelperArgs(pass, fset, call, fn.Body, dm, &issues)
			return true
		})
	}

	return issues
}

func checkDBCallAdvisoryLock(pass *analysis.Pass, fset *token.FileSet, call *ast.CallExpr, body *ast.BlockStmt, dm *directives.DirectiveMap, issues *[]Issue) {
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil || !callsite.IsDBQueryMethod(sel.Sel.Name) {
		return
	}

	// Filter non-db receivers
	if id, ok := sel.X.(*ast.Ident); ok {
		switch strings.ToLower(id.Name) {
		case "log", "logger", "search", "client", "http", "queue", "cmd", "runner", "cache":
			return
		}
	}

	query := extractSQLQueryString(call, body, pass)
	if strings.TrimSpace(query) == "" {
		return
	}
	reportAdvisoryViolations(fset, query, call.Pos(), dm, issues)
}

func extractSQLQueryString(call *ast.CallExpr, body *ast.BlockStmt, pass *analysis.Pass) string {
	sqlArg := callsite.ExtractSQLArg(call, pass)
	if sqlArg == nil {
		if s, ok := callsite.ExtractQueryString(call); ok {
			return s
		}
		return ""
	}

	switch e := sqlArg.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return strings.Trim(e.Value, "`\"")
		}
	case *ast.Ident:
		if constVal, ok := resolveStringConstant(pass, e); ok {
			return constVal
		}
		if body != nil {
			var query string
			ast.Inspect(body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || assign.Pos() >= call.Pos() {
					return true
				}
				for i, lhs := range assign.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && isSameObject(pass, id, e, body) && i < len(assign.Rhs) {
						if lit, ok := assign.Rhs[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							query = strings.Trim(lit.Value, "`\"")
							return false
						}
					}
				}
				return true
			})
			if query != "" {
				return query
			}
		}
	}

	if s, ok := callsite.ExtractQueryString(call); ok {
		return s
	}
	return ""
}

func reportAdvisoryViolations(fset *token.FileSet, query string, pos token.Pos, dm *directives.DirectiveMap, issues *[]Issue) {
	if fset != nil && dm != nil && dm.IsIgnored(fset, pos, RuleCode) {
		return
	}

	violations := InspectAdvisorySQL(query)
	for _, v := range violations {
		switch v.Type {
		case ViolationSessionLock:
			*issues = append(*issues, Issue{
				Pos:     pos,
				Message: fmt.Sprintf("forbidden session-level advisory lock %q; use transaction-scoped \"pg_advisory_xact_lock\" or \"argus.WithAdvisoryLock\" to prevent connection pool leaks", v.FuncName),
			})
		case ViolationHardcodedIntKey:
			*issues = append(*issues, Issue{
				Pos:     pos,
				Message: "hardcoded integer advisory lock key in SQL; use registered namespace constants or argus.LockKey(domain, resource) to prevent collision",
			})
		}
	}
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
