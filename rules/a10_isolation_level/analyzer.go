// Package a10_isolation_level enforces that transactions modifying critical tables
// (saldo, kuota, nomor_urut, rekening) do not rely on default ReadCommitted isolation.
package a10_isolation_level

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A10"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A10.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a10_isolation_level",
	Doc:  "Enforces explicit Serializable/RepeatableRead or row locking on critical table transactions",
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
	customTables := cfg.GetStringSlice(RuleCode, "critical_tables", nil)

	for _, file := range pass.Files {
		pos := pass.Fset.Position(file.Package)
		if strings.HasSuffix(pos.Filename, "_test.go") {
			continue
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			inspectFunctionIsolation(pass, fn.Body, customTables, dm)
		}
	}

	return nil, nil
}

func inspectFunctionIsolation(pass *analysis.Pass, body *ast.BlockStmt, customTables []string, dm *directives.DirectiveMap) {
	// 1. Inspect WithTx helper calls
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		fnName := getCallTargetName(call.Fun)
		if fnName != "WithTx" && !strings.HasSuffix(fnName, ".WithTx") {
			return true
		}

		closure := extractClosureArg(call)
		if closure == nil || closure.Body == nil {
			return true
		}

		hasWrite, hasRowLock, hasAdvisory := analyzeClosureQueries(closure.Body, customTables)
		if !hasWrite || hasRowLock || hasAdvisory || isEnclosedInAdvisory(call, body) {
			return true
		}

		var hasIso bool
		if len(call.Args) >= 4 {
			hasIso = HasStrongIsolation(call.Args[3], body)
		}

		if !hasIso && !dm.IsIgnored(pass.Fset, call.Pos(), RuleCode) {
			pass.Reportf(call.Pos(), "[%s] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock; use pgx.TxOptions or SELECT ... FOR UPDATE", RuleCode)
		}

		return true
	})

	// 2. Inspect explicit transaction blocks (pool.Begin / pool.BeginTx)
	inspectExplicitTxIsolation(pass, body, customTables, dm)
}

func inspectExplicitTxIsolation(pass *analysis.Pass, body *ast.BlockStmt, customTables []string, dm *directives.DirectiveMap) {
	var inTx bool
	var txVarName string
	var beginPos ast.Node
	var hasStrongIso bool
	var hasWrite bool
	var hasRowLock bool
	var hasAdvisory bool

	for _, stmt := range body.List {
		if assign, ok := stmt.(*ast.AssignStmt); ok {
			for i, rhs := range assign.Rhs {
				if call, ok := rhs.(*ast.CallExpr); ok {
					name := getCallTargetName(call.Fun)
					if name == "Begin" || strings.HasSuffix(name, ".Begin") {
						if i < len(assign.Lhs) {
							if id, ok := assign.Lhs[i].(*ast.Ident); ok {
								inTx = true
								txVarName = id.Name
								beginPos = call
								hasStrongIso = false
								hasWrite = false
								hasRowLock = false
								hasAdvisory = false
							}
						}
					} else if name == "BeginTx" || strings.HasSuffix(name, ".BeginTx") {
						if i < len(assign.Lhs) {
							if id, ok := assign.Lhs[i].(*ast.Ident); ok {
								inTx = true
								txVarName = id.Name
								beginPos = call
								hasWrite = false
								hasRowLock = false
								hasAdvisory = false
								if len(call.Args) >= 2 {
									hasStrongIso = HasStrongIsolation(call.Args[1], body)
								}
							}
						}
					}
				}
			}
		}

		if inTx {
			if _, isDefer := stmt.(*ast.DeferStmt); isDefer {
				continue
			}

			ast.Inspect(stmt, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
					return true
				}
				query, found := callsite.ExtractQueryString(call)
				if !found {
					return true
				}
				if IsCriticalTableWrite(query, customTables) {
					hasWrite = true
				}
				if HasPessimisticRowLock(query) {
					hasRowLock = true
				}
				if HasAdvisoryLockCall(query) {
					hasAdvisory = true
				}
				return true
			})

			if isTxEndStmt(stmt, txVarName) {
				if hasWrite && !hasStrongIso && !hasRowLock && !hasAdvisory {
					if beginPos != nil && !dm.IsIgnored(pass.Fset, beginPos.Pos(), RuleCode) {
						pass.Reportf(beginPos.Pos(), "[%s] transaction writing to critical table without explicit Serializable/RepeatableRead isolation level or row lock; use pgx.TxOptions or SELECT ... FOR UPDATE", RuleCode)
					}
				}
				inTx = false
				txVarName = ""
				beginPos = nil
			}
		}
	}
}

func analyzeClosureQueries(body *ast.BlockStmt, customTables []string) (hasWrite, hasRowLock, hasAdvisory bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
			return true
		}
		query, found := callsite.ExtractQueryString(call)
		if !found {
			return true
		}
		if IsCriticalTableWrite(query, customTables) {
			hasWrite = true
		}
		if HasPessimisticRowLock(query) {
			hasRowLock = true
		}
		if HasAdvisoryLockCall(query) {
			hasAdvisory = true
		}
		return true
	})
	return
}

func isEnclosedInAdvisory(call *ast.CallExpr, body *ast.BlockStmt) bool {
	var enclosed bool
	ast.Inspect(body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := getCallTargetName(c.Fun)
		if name == "WithAdvisoryLock" || strings.HasSuffix(name, ".WithAdvisoryLock") {
			for _, arg := range c.Args {
				if lit, ok := arg.(*ast.FuncLit); ok && lit.Body != nil {
					ast.Inspect(lit.Body, func(in ast.Node) bool {
						if in == call {
							enclosed = true
							return false
						}
						return true
					})
				}
			}
		}
		return true
	})
	return enclosed
}
