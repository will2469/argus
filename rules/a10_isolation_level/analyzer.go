// Package a10_isolation_level enforces that transactions modifying critical tables
// (saldo, kuota, nomor_urut, rekening) do not rely on default ReadCommitted isolation.
package a10_isolation_level

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const (
	RuleCode        = "ARGUS-A10"
	violationMsgFmt = "transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock; use pgx.TxOptions or SELECT ... FOR UPDATE"
)

// Analyzer defines the analysis.Analyzer for rule ARGUS-A10.
var Analyzer = &analysis.Analyzer{
	Name: "a10",
	Doc:  "Enforces explicit Serializable/RepeatableRead or row locking on critical table transactions",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A10.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for unsafe isolation level transactions on critical tables.
// Can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap, customTables []string) []Issue {
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

		inspectFunctionIsolation(pass, fset, fn, file, customTables, dm, &issues)
	}

	return issues
}

func inspectFunctionIsolation(pass *analysis.Pass, fset *token.FileSet, fn *ast.FuncDecl, file *ast.File, customTables []string, dm *directives.DirectiveMap, issues *[]Issue) {
	if fn == nil || fn.Body == nil {
		return
	}

	// 1. Inspect WithTx helper calls
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		fnName := callsite.GetCallMethodName(call.Fun)
		if fnName != "WithTx" && fnName != "BeginFunc" && fnName != "ExecuteTx" {
			return true
		}
		if !isDBReceiver(pass, call.Fun, fn, file) && !isWithTxHelperCall(call) {
			return true
		}

		closure := extractClosureArg(call)
		if closure == nil || closure.Body == nil {
			return true
		}

		writtenCritical, lockedTables, advisoryCalls := analyzeClosureQueries(pass, closure, customTables)
		if len(writtenCritical) == 0 {
			return true
		}

		var hasIso bool
		if len(call.Args) >= 4 {
			hasIso = HasStrongIsolation(pass, call.Args[3], fn.Body)
		}
		if hasIso {
			return true
		}

		allProtected := true
		for _, target := range writtenCritical {
			if !isTableProtected(target, lockedTables, advisoryCalls) && !isEnclosedInCorrelatedAdvisory(call, fn.Body, target) {
				allProtected = false
				break
			}
		}

		if !allProtected && (fset == nil || dm == nil || !dm.IsIgnored(fset, call.Pos(), RuleCode)) {
			*issues = append(*issues, Issue{
				Pos:     call.Pos(),
				Message: violationMsgFmt,
			})
		}

		return true
	})

	// 2. Inspect explicit transaction blocks (pool.Begin / pool.BeginTx)
	inspectExplicitTxIsolation(pass, fset, fn, file, customTables, dm, issues)
}

func isWithTxHelperCall(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	name := callsite.GetCallMethodName(call.Fun)
	return (name == "WithTx" || name == "BeginFunc" || name == "ExecuteTx") && len(call.Args) >= 2
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)
	customTables := cfg.GetStringSlice(RuleCode, "critical_tables", nil)

	for _, file := range pass.Files {
		issues := InspectFile(pass, pass.Fset, file, dm, customTables)
		for _, iss := range issues {
			pass.Reportf(iss.Pos, "[%s] %s", RuleCode, iss.Message)
		}
	}

	return nil, nil
}
