// Package a10_isolation_level provides AST transaction walkers for explicit blocks and closures.
package a10_isolation_level

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/directives"
)

func inspectExplicitTxIsolation(pass *analysis.Pass, fset *token.FileSet, body *ast.BlockStmt, customTables []string, dm *directives.DirectiveMap, issues *[]Issue) {
	var inTx bool
	var txVarName string
	var beginPos ast.Node
	var hasStrongIso bool
	var writtenCritical []TableRef
	var lockedTables []TableRef
	var advisoryCalls []string

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
								writtenCritical = nil
								lockedTables = nil
								advisoryCalls = nil
							}
						}
					} else if name == "BeginTx" || strings.HasSuffix(name, ".BeginTx") {
						if i < len(assign.Lhs) {
							if id, ok := assign.Lhs[i].(*ast.Ident); ok {
								inTx = true
								txVarName = id.Name
								beginPos = call
								writtenCritical = nil
								lockedTables = nil
								advisoryCalls = nil
								if len(call.Args) >= 2 {
									hasStrongIso = HasStrongIsolation(call.Args[1], body)
								} else {
									hasStrongIso = false
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
				if id, ok := sel.X.(*ast.Ident); ok {
					switch strings.ToLower(id.Name) {
					case "log", "logger", "search", "client", "http", "queue", "cmd", "runner", "cache":
						return true
					}
				}
				queries := extractTxQueryStrings(call, body, pass)
				for _, query := range queries {
					if ref, ok := ExtractCriticalWriteTable(query, customTables); ok {
						writtenCritical = append(writtenCritical, ref)
					}
					lockedTables = append(lockedTables, ExtractLockedTables(query)...)
					if adv := ExtractAdvisoryLockTarget(query); adv != "" {
						advisoryCalls = append(advisoryCalls, adv)
					}
				}
				return true
			})

			if isTxEndStmt(stmt, txVarName) {
				if len(writtenCritical) > 0 && !hasStrongIso {
					for _, target := range writtenCritical {
						if !isTableProtected(target, lockedTables, advisoryCalls) {
							if beginPos != nil && (fset == nil || dm == nil || !dm.IsIgnored(fset, beginPos.Pos(), RuleCode)) {
								*issues = append(*issues, Issue{
									Pos:     beginPos.Pos(),
									Message: violationMsgFmt,
								})
								break
							}
						}
					}
				}
				inTx = false
				txVarName = ""
				beginPos = nil
			}
		}
	}
}

func analyzeClosureQueries(pass *analysis.Pass, body *ast.BlockStmt, customTables []string) (writtenCritical []TableRef, lockedTables []TableRef, advisoryCalls []string) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			switch strings.ToLower(id.Name) {
			case "log", "logger", "search", "client", "http", "queue", "cmd", "runner", "cache":
				return true
			}
		}
		queries := extractTxQueryStrings(call, body, pass)
		for _, query := range queries {
			if ref, ok := ExtractCriticalWriteTable(query, customTables); ok {
				writtenCritical = append(writtenCritical, ref)
			}
			lockedTables = append(lockedTables, ExtractLockedTables(query)...)
			if adv := ExtractAdvisoryLockTarget(query); adv != "" {
				advisoryCalls = append(advisoryCalls, adv)
			}
		}
		return true
	})
	return
}

func extractTxQueryStrings(call *ast.CallExpr, body *ast.BlockStmt, pass *analysis.Pass) []string {
	if call == nil || len(call.Args) == 0 {
		return nil
	}

	sqlArg := callsite.ExtractSQLArg(call, pass)
	if sqlArg == nil {
		return nil
	}

	var results []string
	switch e := sqlArg.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			results = append(results, strings.Trim(e.Value, "`\""))
		}
	case *ast.Ident:
		if body != nil {
			ast.Inspect(body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || assign.Pos() >= call.Pos() {
					return true
				}
				for i, lhs := range assign.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name == e.Name && i < len(assign.Rhs) {
						if lit, ok := assign.Rhs[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							results = append(results, strings.Trim(lit.Value, "`\""))
						}
					}
				}
				return true
			})
		}
	}

	if len(results) == 0 {
		if s, ok := callsite.ExtractQueryString(call); ok {
			results = append(results, s)
		}
	}

	return results
}
