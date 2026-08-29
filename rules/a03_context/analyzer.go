// Package a03_context detects database operations executed with raw unbounded contexts
// such as context.Background() or context.TODO(), enforcing request deadlines or timeouts.
package a03_context

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A03"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A03.
var Analyzer = &analysis.Analyzer{
	Name: "a03",
	Doc:  "Prohibits raw context.Background() or context.TODO() in database operations",
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
			continue // Test files are exempted from strict context bounds
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}

				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !isDBContextMethod(sel) {
					return true
				}

				ctxArg := call.Args[0]
				if dm.IsIgnored(pass.Fset, ctxArg.Pos(), RuleCode) {
					return true
				}

				if IsRawContext(ctxArg, fn.Body) {
					pass.Reportf(ctxArg.Pos(), "[%s] database operation %s executed with unbounded context; use bounded context (r.Context() or context.WithTimeout)", RuleCode, sel.Sel.Name)
				}
				return true
			})
		}
	}

	return nil, nil
}

func isDBContextMethod(sel *ast.SelectorExpr) bool {
	// Exclude HTTP URL query calls (r.URL.Query())
	if innerSel, ok := sel.X.(*ast.SelectorExpr); ok && innerSel.Sel.Name == "URL" {
		return false
	}

	switch sel.Sel.Name {
	case "Query", "QueryRow", "Exec", "Begin", "BeginTx", "SendBatch", "CopyFrom", "Ping":
		return true
	}
	return false
}
