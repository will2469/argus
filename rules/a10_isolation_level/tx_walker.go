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

func inspectExplicitTxIsolation(pass *analysis.Pass, fset *token.FileSet, fn *ast.FuncDecl, file *ast.File, customTables []string, dm *directives.DirectiveMap, issues *[]Issue) {
	if fn == nil || fn.Body == nil {
		return
	}

	var inTx bool
	var txID *ast.Ident
	var txVarName string
	var beginPos ast.Node
	var hasStrongIso bool
	var writtenCritical []TableRef
	var lockedTables []TableRef
	var advisoryCalls []string

	for _, stmt := range fn.Body.List {
		if assign, ok := stmt.(*ast.AssignStmt); ok {
			for i, rhs := range assign.Rhs {
				if call, ok := rhs.(*ast.CallExpr); ok {
					name := callsite.GetCallMethodName(call.Fun)
					if (name == "Begin" || name == "BeginTx") && isDBReceiver(pass, call.Fun, fn, file) {
						targetIdx := 0
						if i < len(assign.Lhs) {
							targetIdx = i
						}
						if len(assign.Lhs) > targetIdx {
							if id, ok := assign.Lhs[targetIdx].(*ast.Ident); ok {
								inTx = true
								txID = id
								txVarName = id.Name
								beginPos = call
								writtenCritical = nil
								lockedTables = nil
								advisoryCalls = nil
								if name == "BeginTx" && len(call.Args) >= 2 {
									hasStrongIso = HasStrongIsolation(pass, call.Args[1], fn.Body)
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
				if !isProvenTxReceiver(pass, sel.X, txID) {
					return true
				}

				queries := extractTxQueryStrings(call, fn.Body, pass)
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
				txID = nil
				txVarName = ""
				beginPos = nil
			}
		}
	}
}

func analyzeClosureQueries(pass *analysis.Pass, closure *ast.FuncLit, customTables []string) (writtenCritical []TableRef, lockedTables []TableRef, advisoryCalls []string) {
	if closure == nil || closure.Body == nil {
		return
	}

	var txParam *ast.Ident
	if closure.Type != nil && closure.Type.Params != nil && len(closure.Type.Params.List) > 0 {
		firstParam := closure.Type.Params.List[0]
		if len(firstParam.Names) > 0 {
			txParam = firstParam.Names[0]
		}
	}

	ast.Inspect(closure.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
			return true
		}
		if !isProvenTxReceiver(pass, sel.X, txParam) {
			return true
		}

		queries := extractTxQueryStrings(call, closure.Body, pass)
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
		if constVal, ok := resolveStringConstant(pass, e); ok {
			results = append(results, constVal)
			return results
		}
		if body != nil {
			var latestVal string
			var found bool
			ast.Inspect(body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || assign.Pos() >= call.Pos() {
					return true
				}
				for i, lhs := range assign.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && isSameObject(pass, id, e) && i < len(assign.Rhs) {
						if lit, ok := assign.Rhs[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							latestVal = strings.Trim(lit.Value, "`\"")
							found = true
						}
					}
				}
				return true
			})
			if found {
				results = append(results, latestVal)
				return results
			}
		}
	}

	if len(results) == 0 {
		if s, ok := callsite.ExtractQueryString(call); ok {
			results = append(results, s)
		}
	}

	return results
}
