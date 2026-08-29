// Package a12_timeout_config enforces explicit server-side and client-side timeout configurations
// (statement_timeout, lock_timeout, idle_in_transaction, MaxConnIdleTime, MaxConnLifetime) on pgxpool initialization.
package a12_timeout_config

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

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	for _, file := range pass.Files {
		pos := pass.Fset.Position(file.Package)
		if strings.HasSuffix(pos.Filename, "_test.go") {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				inspectCall(pass, node, file, dm)
			case *ast.CompositeLit:
				inspectCompositeLit(pass, node, file, dm)
			}
			return true
		})
	}

	return nil, nil
}

func inspectCall(pass *analysis.Pass, call *ast.CallExpr, file *ast.File, dm *directives.DirectiveMap) {
	methodName := callsite.GetCallMethodName(call.Fun)

	switch methodName {
	case "New", "pgxpool.New":
		// pgxpool.New(ctx, dsn)
		dsnArgIdx := 0
		if len(call.Args) >= 2 {
			dsnArgIdx = 1
		}
		if dsnArgIdx < len(call.Args) {
			dsnStr, ok := callsite.ExtractQueryString(call)
			if ok {
				checkDSNCall(pass, call.Args[dsnArgIdx], dsnStr, dm)
			}
		}
	case "NewWithConfig", "pgxpool.NewWithConfig":
		// pgxpool.NewWithConfig(ctx, cfg)
		cfgArgIdx := 0
		if len(call.Args) >= 2 {
			cfgArgIdx = 1
		}
		if cfgArgIdx < len(call.Args) {
			checkNewWithConfigCall(pass, call, call.Args[cfgArgIdx], file, dm)
		}
	}
}

func checkDSNCall(pass *analysis.Pass, dsnExpr ast.Expr, dsn string, dm *directives.DirectiveMap) {
	if dm != nil && dm.IsIgnored(pass.Fset, dsnExpr.Pos(), RuleCode) {
		return
	}

	res := CheckDSN(dsn)
	for _, missing := range res.Missing {
		pass.Reportf(dsnExpr.Pos(),
			"[%s] pgxpool DSN missing '%s' parameter; add '%s=<duration>' to prevent unbounded resource consumption",
			RuleCode, missing, missing)
	}
	for _, zero := range res.Zero {
		pass.Reportf(dsnExpr.Pos(),
			"[%s] pgxpool DSN parameter '%s' must not be set to 0 (unlimited)",
			RuleCode, zero)
	}
}

func checkNewWithConfigCall(pass *analysis.Pass, call *ast.CallExpr, cfgArg ast.Expr, file *ast.File, dm *directives.DirectiveMap) {
	if dm != nil && dm.IsIgnored(pass.Fset, call.Pos(), RuleCode) {
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
	reportConfigStatus(pass, call.Pos(), status, dm)
}

func inspectCompositeLit(pass *analysis.Pass, lit *ast.CompositeLit, file *ast.File, dm *directives.DirectiveMap) {
	if !isPgxpoolConfigType(pass, lit.Type) {
		return
	}
	if enclosing := findEnclosingFunc(file, lit.Pos()); enclosing != nil {
		if enclosing.Name.Name == "ParseConfig" || enclosing.Name.Name == "DefaultConfig" {
			return
		}
	}
	if dm != nil && dm.IsIgnored(pass.Fset, lit.Pos(), RuleCode) {
		return
	}

	status := EvalCompositeLit(lit)
	reportConfigStatus(pass, lit.Pos(), status, dm)
}

func reportConfigStatus(pass *analysis.Pass, pos token.Pos, status ConfigStatus, dm *directives.DirectiveMap) {
	report := func(clause, msg string) {
		fullCode := fmt.Sprintf("%s.%s", RuleCode, clause)
		if dm != nil && (dm.IsIgnored(pass.Fset, pos, RuleCode) || dm.IsIgnored(pass.Fset, pos, fullCode)) {
			return
		}
		pass.Reportf(pos, "[%s] %s", RuleCode, msg)
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

func isPgxpoolConfigType(pass *analysis.Pass, expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "pgxpool" && sel.Sel.Name == "Config" {
			return true
		}
	}
	if id, ok := expr.(*ast.Ident); ok && id.Name == "Config" {
		if pass != nil && pass.Pkg != nil && (pass.Pkg.Name() == "a12" || pass.Pkg.Name() == "a") {
			return true
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
