// Package a03_context detects database operations executed with raw unbounded contexts
// such as context.Background() or context.TODO(), enforcing request deadlines or timeouts.
package a03_context

import (
	"fmt"
	"go/ast"
	"go/token"
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

// Issue describes a detected violation of ARGUS-A03.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for database operations using raw unbounded contexts.
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
		return nil // Test files are exempted from strict context bounds
	}

	var issues []Issue
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		inspectFunctionBody(fset, fn.Body, dm, &issues)
	}
	return issues
}

func inspectFunctionBody(fset *token.FileSet, body *ast.BlockStmt, dm *directives.DirectiveMap, issues *[]Issue) {
	ast.Inspect(body, func(n ast.Node) bool {
		// Inspect nested closures separately
		if lit, ok := n.(*ast.FuncLit); ok && lit.Body != nil {
			inspectFunctionBody(fset, lit.Body, dm, issues)
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !isDBContextMethod(sel) {
			return true
		}

		ctxArg := call.Args[0]
		if fset != nil && dm != nil && dm.IsIgnored(fset, ctxArg.Pos(), RuleCode) {
			return true
		}

		if IsRawContext(ctxArg, body) {
			msg := fmt.Sprintf("database operation %s executed with unbounded context; use bounded context (r.Context() or context.WithTimeout)", sel.Sel.Name)
			*issues = append(*issues, Issue{
				Pos:     ctxArg.Pos(),
				Message: msg,
			})
		}
		return true
	})
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

func isDBContextMethod(sel *ast.SelectorExpr) bool {
	// Exclude HTTP URL query calls (r.URL.Query())
	if innerSel, ok := sel.X.(*ast.SelectorExpr); ok && innerSel.Sel.Name == "URL" {
		return false
	}

	if id, ok := sel.X.(*ast.Ident); ok {
		lower := strings.ToLower(id.Name)
		switch lower {
		case "search", "client", "http", "httpclient", "logger", "log", "queue", "cmd", "url", "req", "response":
			return false
		}
	}

	switch sel.Sel.Name {
	case "Query", "QueryRow", "Exec", "Begin", "BeginTx", "SendBatch", "CopyFrom", "Ping":
		return true
	}
	return false
}
