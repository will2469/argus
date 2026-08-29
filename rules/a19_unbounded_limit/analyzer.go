// Package a19_unbounded_limit detects and flags queries without LIMIT clauses on high-cardinality tables
// to prevent buffer cache pollution and Go runtime OOM crashes (CWE-400).
package a19_unbounded_limit

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A19.
const RuleCode = "ARGUS-A19"

// Analyzer defines the analysis.Analyzer for ARGUS-A19.
var Analyzer = &analysis.Analyzer{
	Name: "a19",
	Doc:  "Enforce explicit LIMIT or keyset pagination on high-cardinality tables to prevent buffer pool eviction and OOM crashes (CWE-400)",
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
	tableMap := GetHighCardinalityTables(cfg)
	keyColumnMap := GetKeyColumns(cfg)

	for _, file := range pass.Files {
		pos := pass.Fset.Position(file.Package)
		if strings.HasSuffix(pos.Filename, "_test.go") {
			continue
		}

		InspectFile(file, pass.Fset, dm, tableMap, keyColumnMap, func(pos token.Pos, format string, args ...any) {
			pass.Reportf(pos, format, args...)
		})
	}

	return nil, nil
}

// InspectFile walks an AST file and reports violations of ARGUS-A19.
func InspectFile(file *ast.File, fset *token.FileSet, dm *directives.DirectiveMap, tableMap map[string]bool, keyColumnMap map[string]bool, report func(pos token.Pos, format string, args ...any)) {
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
		if !callsite.IsDBQueryMethod(methodName) {
			return true
		}

		query, found := callsite.ExtractQueryString(call)
		if !found || strings.TrimSpace(query) == "" {
			return true
		}

		if unbounded, table := CheckUnboundedQuery(query, tableMap, keyColumnMap); unbounded {
			var queryIdentName string
			for _, arg := range call.Args {
				if id, ok := arg.(*ast.Ident); ok {
					if id.Name == "ctx" || id.Name == "c" || id.Name == "context" {
						continue
					}
					queryIdentName = id.Name
					break
				}
			}

			if queryIdentName != "" && currentFunc != nil && hasLimitInFunction(currentFunc, queryIdentName) {
				return true
			}

			if dm != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".UNBOUNDED-LIMIT")) {
				return true
			}
			report(call.Pos(),
				"[%s] unbounded query on high-cardinality table %q without LIMIT or keyset pagination; risk of buffer cache eviction and OOM crash (CWE-400)",
				RuleCode, table)
		}

		return true
	})
}

func hasLimitInFunction(fn ast.Node, queryVarName string) bool {
	if fn == nil || queryVarName == "" {
		return false
	}

	foundLimit := false
	ast.Inspect(fn, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == queryVarName {
					for _, rhs := range assign.Rhs {
						if containsLimitKeyword(rhs) {
							foundLimit = true
							return false
						}
					}
				}
			}
		}
		return true
	})
	return foundLimit
}

func containsLimitKeyword(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return limitRegex.MatchString(e.Value)
	case *ast.BinaryExpr:
		return containsLimitKeyword(e.X) || containsLimitKeyword(e.Y)
	case *ast.CallExpr:
		for _, arg := range e.Args {
			if containsLimitKeyword(arg) {
				return true
			}
		}
	}
	return false
}
