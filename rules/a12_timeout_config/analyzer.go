// Package a12_timeout_config enforces explicit server-side and client-side timeout configurations
// (statement_timeout, lock_timeout, idle_in_transaction, MaxConnIdleTime, MaxConnLifetime) on pgxpool initialization.
package a12_timeout_config

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A12.
const RuleCode = "ARGUS-A12"

// Analyzer defines the analysis.Analyzer for ARGUS-A12.
var Analyzer = &analysis.Analyzer{
	Name: "a12",
	Doc:  "Enforce explicit server-side timeout configuration (statement_timeout, lock_timeout, idle_in_transaction) on pgxpool initialization",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A12.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for missing or zero timeouts on pgxpool initialization.
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
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			inspectCall(fset, node, file, dm, &issues)
		case *ast.CompositeLit:
			inspectCompositeLit(pass, fset, node, file, dm, &issues)
		}
		return true
	})

	return issues
}

func inspectCall(fset *token.FileSet, call *ast.CallExpr, file *ast.File, dm *directives.DirectiveMap, issues *[]Issue) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	if id, ok := sel.X.(*ast.Ident); ok && id.Name != "pgxpool" {
		return
	}

	methodName := sel.Sel.Name

	switch methodName {
	case "New":
		// pgxpool.New(ctx, dsn)
		dsnArgIdx := 0
		if len(call.Args) >= 2 {
			dsnArgIdx = 1
		}
		if dsnArgIdx < len(call.Args) {
			dsnStrings := extractAllDSNStrings(call, file)
			for _, dsnStr := range dsnStrings {
				checkDSNCall(fset, call.Args[dsnArgIdx], dsnStr, dm, issues)
			}
		}
	case "NewWithConfig":
		// pgxpool.NewWithConfig(ctx, cfg)
		cfgArgIdx := 0
		if len(call.Args) >= 2 {
			cfgArgIdx = 1
		}
		if cfgArgIdx < len(call.Args) {
			checkNewWithConfigCall(fset, call, call.Args[cfgArgIdx], file, dm, issues)
		}
	}
}

func checkNewWithConfigCall(fset *token.FileSet, call *ast.CallExpr, cfgArg ast.Expr, file *ast.File, dm *directives.DirectiveMap, issues *[]Issue) {
	if fset != nil && dm != nil && dm.IsIgnored(fset, call.Pos(), RuleCode) {
		return
	}

	id, ok := cfgArg.(*ast.Ident)
	if !ok {
		return
	}

	enclosingFunc := findEnclosingFunc(file, call.Pos())
	if enclosingFunc == nil || enclosingFunc.Body == nil {
		return
	}

	status := EvalBlockAssignments(enclosingFunc.Body, id.Name)
	reportConfigStatus(fset, call.Pos(), status, dm, issues)
}

func inspectCompositeLit(pass *analysis.Pass, fset *token.FileSet, lit *ast.CompositeLit, file *ast.File, dm *directives.DirectiveMap, issues *[]Issue) {
	if !isPgxpoolConfigType(pass, file, lit.Type) {
		return
	}
	if enclosing := findEnclosingFunc(file, lit.Pos()); enclosing != nil {
		if enclosing.Name.Name == "ParseConfig" || enclosing.Name.Name == "DefaultConfig" {
			return
		}
	}
	if fset != nil && dm != nil && dm.IsIgnored(fset, lit.Pos(), RuleCode) {
		return
	}

	status := EvalCompositeLit(lit)
	reportConfigStatus(fset, lit.Pos(), status, dm, issues)
}

func reportConfigStatus(fset *token.FileSet, pos token.Pos, status ConfigStatus, dm *directives.DirectiveMap, issues *[]Issue) {
	report := func(clause, msg string) {
		fullCode := fmt.Sprintf("%s.%s", RuleCode, clause)
		if fset != nil && dm != nil && (dm.IsIgnored(fset, pos, RuleCode) || dm.IsIgnored(fset, pos, fullCode)) {
			return
		}
		*issues = append(*issues, Issue{
			Pos:     pos,
			Message: msg,
		})
	}

	if !status.HasStatementTimeout {
		report("statement-timeout", "pgxpool.Config missing ConnConfig.RuntimeParams[\"statement_timeout\"]; set to prevent unbounded query execution")
	}
	if !status.HasLockTimeout {
		report("lock-timeout", "pgxpool.Config missing ConnConfig.RuntimeParams[\"lock_timeout\"]; set to prevent lock acquisition starvation")
	}
	if !status.HasMaxConnIdleTime {
		report("max-conn-idle-time", "pgxpool.Config missing MaxConnIdleTime; set to prevent idle connection accumulation")
	}
	if !status.HasMaxConnLifetime {
		report("max-conn-lifetime", "pgxpool.Config missing MaxConnLifetime; set to prevent long-lived connections accumulating issues")
	}
	if status.HasZeroTimeout {
		report("zero-timeout", fmt.Sprintf("pgxpool.Config timeout parameter '%s' must not be set to 0 (unlimited)", status.ZeroTimeoutParam))
	}
}

func isPgxpoolConfigType(pass *analysis.Pass, file *ast.File, expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "pgxpool" && sel.Sel.Name == "Config" {
			return true
		}
	}
	if id, ok := expr.(*ast.Ident); ok && id.Name == "Config" {
		if pass != nil && pass.Pkg != nil {
			pkg := pass.Pkg.Name()
			if pkg == "a12" || pkg == "a" || pkg == "positive" || pkg == "adversarial" || pkg == "negative" {
				return true
			}
		}
		if file != nil {
			pkg := file.Name.Name
			if pkg == "a12" || pkg == "positive" || pkg == "adversarial" || pkg == "negative" {
				return true
			}
		}
	}
	return false
}

func findEnclosingFunc(file *ast.File, pos token.Pos) *ast.FuncDecl {
	var enclosing *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Pos() <= pos && pos <= fn.End() {
				enclosing = fn
				break
			}
		}
	}
	return enclosing
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
