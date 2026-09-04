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
			inspectCall(pass, fset, node, file, dm, &issues)
		case *ast.CompositeLit:
			inspectCompositeLit(pass, fset, node, file, dm, &issues)
		}
		return true
	})

	return issues
}

func inspectCall(pass *analysis.Pass, fset *token.FileSet, call *ast.CallExpr, file *ast.File, dm *directives.DirectiveMap, issues *[]Issue) {
	isPgx, methodName := isPgxpoolCall(pass, file, call)
	if !isPgx {
		return
	}

	switch methodName {
	case "New":
		arg, _ := findCallArg(call, pass)
		if arg != nil {
			dsnStrings := extractAllDSNStrings(call, file, pass)
			for _, dsnStr := range dsnStrings {
				checkDSNCall(fset, arg, dsnStr, dm, issues)
			}
		}
	case "NewWithConfig":
		arg, _ := findCallArg(call, pass)
		if arg != nil {
			checkNewWithConfigCall(pass, fset, call, arg, file, dm, issues)
		}
	}
}

func checkNewWithConfigCall(pass *analysis.Pass, fset *token.FileSet, call *ast.CallExpr, cfgArg ast.Expr, file *ast.File, dm *directives.DirectiveMap, issues *[]Issue) {
	if fset != nil && dm != nil && dm.IsIgnored(fset, call.Pos(), RuleCode) {
		return
	}

	status := EvalConfigFlow(pass, file, cfgArg, call)
	reportConfigStatus(fset, call.Pos(), status, dm, issues)
}

func inspectCompositeLit(pass *analysis.Pass, fset *token.FileSet, lit *ast.CompositeLit, file *ast.File, dm *directives.DirectiveMap, issues *[]Issue) {
	expr := lit.Type
	if expr == nil {
		expr = lit
	}
	if !isPgxpoolConfigType(pass, file, expr) {
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
	if !status.HasIdleInTransaction {
		report("idle-in-transaction", "pgxpool.Config missing ConnConfig.RuntimeParams[\"idle_in_transaction_session_timeout\"]; set to prevent idle transactions holding locks")
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
