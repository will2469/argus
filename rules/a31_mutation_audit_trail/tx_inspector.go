// Package a31_mutation_audit_trail inspects functions and transaction scopes
// for unlogged database mutations.
package a31_mutation_audit_trail

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/directives"
)

func inspectFunction(
	pass *analysis.Pass,
	fset *token.FileSet,
	fn *ast.FuncDecl,
	funcDecls []*ast.FuncDecl,
	dm *directives.DirectiveMap,
	auditMethods []string,
	exemptTables []string,
	issues *[]Issue,
) {
	if fn == nil || fn.Body == nil {
		return
	}

	// 1. Inspect closure-based transactions (BeginFunc, ExecuteTx, WithTx, etc.)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		closure := extractTxClosure(call)
		if closure != nil && closure.Body != nil {
			collector := &txAuditCollector{}
			visited := make(map[*ast.FuncDecl]bool)

			walkTxStatements(pass, closure.Body.List, funcDecls, visited, auditMethods, exemptTables, collector)

			if collector.hasNonExemptMutation && !collector.hasAudit && collector.firstMutationPos != nil {
				mutPos := collector.firstMutationPos.Pos()
				if dm != nil && fset != nil {
					if dm.IsIgnored(fset, mutPos, RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode) {
						return true
					}
				}
				*issues = append(*issues, Issue{
					Pos:     mutPos,
					Message: "database mutation inside transaction is missing required audit trail logging; sensitive state changes must be accompanied by an audit record",
				})
			}
		}
		return true
	})

	// 2. Inspect explicit transaction lifecycles (Begin / BeginTx ... Commit)
	inspectExplicitTransactions(pass, fset, fn, funcDecls, dm, auditMethods, exemptTables, issues)
}

func extractTxClosure(call *ast.CallExpr) *ast.FuncLit {
	if call == nil {
		return nil
	}

	methodName := callsite.GetCallMethodName(call.Fun)
	if isTxCoordinatorMethod(methodName) {
		for _, arg := range call.Args {
			if lit, ok := arg.(*ast.FuncLit); ok {
				return lit
			}
		}
	}
	return nil
}

func isTxCoordinatorMethod(name string) bool {
	switch name {
	case "BeginFunc", "ExecuteTx", "ExecuteLockedTx", "WithTx", "RunInTx",
		"BeginTx", "Transaction", "InTx", "WithinTx", "DoInTx":
		return true
	}
	lower := strings.ToLower(name)
	return strings.Contains(lower, "tx") && (strings.HasPrefix(lower, "exec") ||
		strings.HasPrefix(lower, "run") ||
		strings.HasPrefix(lower, "with") ||
		strings.HasPrefix(lower, "begin"))
}

func inspectExplicitTransactions(
	pass *analysis.Pass,
	fset *token.FileSet,
	fn *ast.FuncDecl,
	funcDecls []*ast.FuncDecl,
	dm *directives.DirectiveMap,
	auditMethods []string,
	exemptTables []string,
	issues *[]Issue,
) {
	if fn.Body == nil {
		return
	}

	// Scan statements for explicit Begin/BeginTx calls
	for idx, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}

		isBegin := false
		for _, rhs := range assign.Rhs {
			if call, isCall := rhs.(*ast.CallExpr); isCall {
				mName := callsite.GetCallMethodName(call.Fun)
				if mName == "Begin" || mName == "BeginTx" {
					isBegin = true
					break
				}
			}
		}

		if !isBegin {
			continue
		}

		// Found explicit transaction begin; inspect subsequent statements in this block
		collector := &txAuditCollector{}
		visited := make(map[*ast.FuncDecl]bool)

		subsequentStmts := fn.Body.List[idx+1:]
		walkTxStatements(pass, subsequentStmts, funcDecls, visited, auditMethods, exemptTables, collector)

		if collector.hasNonExemptMutation && !collector.hasAudit && collector.firstMutationPos != nil {
			mutPos := collector.firstMutationPos.Pos()
			if dm != nil && fset != nil {
				if dm.IsIgnored(fset, mutPos, RuleCode) || dm.IsIgnored(fset, assign.Pos(), RuleCode) {
					continue
				}
			}
			*issues = append(*issues, Issue{
				Pos:     mutPos,
				Message: "database mutation inside transaction is missing required audit trail logging; sensitive state changes must be accompanied by an audit record",
			})
		}
	}
}
