// Package a06_runtime_ddl prohibits executing DDL statements in Go application runtime code.
package a06_runtime_ddl

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

const RuleCode = "ARGUS-A06"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A06.
var Analyzer = &analysis.Analyzer{
	Name: "a06",
	Doc:  "Prohibits execution of DDL statements (CREATE, ALTER, DROP, TRUNCATE, GRANT, REVOKE) in runtime Go application code",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A06.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for runtime DDL execution.
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

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
				return true
			}

			// Filter non-db receivers
			if id, ok := sel.X.(*ast.Ident); ok {
				switch strings.ToLower(id.Name) {
				case "log", "logger", "search", "client", "http", "queue", "cmd", "runner", "cache":
					return true
				}
			}

			if dm != nil && fset != nil && dm.IsIgnored(fset, call.Pos(), RuleCode) {
				return true
			}

			// 1. Static AST detection on literal query strings
			if query, found := callsite.ExtractQueryString(call); found && strings.TrimSpace(query) != "" {
				if ddlOp := DetectDDLFromAST(query); ddlOp != "" {
					issues = append(issues, Issue{
						Pos:     call.Pos(),
						Message: fmt.Sprintf("runtime database query contains forbidden DDL statement (%s); DDL is restricted to migrations", ddlOp),
					})
					return true
				}
			}

			// 2. Dynamic DDL detection on all arguments of the call
			for _, arg := range call.Args {
				if ddlOp := DetectDynamicDDL(arg, fn.Body); ddlOp != "" {
					issues = append(issues, Issue{
						Pos:     call.Pos(),
						Message: fmt.Sprintf("runtime database query contains forbidden DDL statement (%s); DDL is restricted to migrations", ddlOp),
					})
					return true
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
