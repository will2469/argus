// Package a12_timeout_config tracks pgxpool.Config initialization and AST assignment flows.
package a12_timeout_config

import (
	"go/ast"
	"go/token"
	"strings"
)

// ConfigStatus tracks presence of required timeout configurations.
type ConfigStatus struct {
	HasStatementTimeout bool
	HasLockTimeout      bool
	HasMaxConnIdleTime  bool
	HasMaxConnLifetime  bool
	HasZeroTimeout      bool
	ZeroTimeoutParam    string
}

// EvalCompositeLit evaluates a pgxpool.Config composite literal.
func EvalCompositeLit(lit *ast.CompositeLit) ConfigStatus {
	var status ConfigStatus

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "MaxConnIdleTime":
			status.HasMaxConnIdleTime = true
		case "MaxConnLifetime":
			status.HasMaxConnLifetime = true
		case "ConnConfig":
			evalConnConfigComposite(kv.Value, &status)
		}
	}

	return status
}

func evalConnConfigComposite(expr ast.Expr, status *ConfigStatus) {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		if key.Name == "RuntimeParams" || key.Name == "Config" {
			evalRuntimeParamsExpr(kv.Value, status)
		}
	}
}

func evalRuntimeParamsExpr(expr ast.Expr, status *ConfigStatus) {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		paramKey := extractLitString(kv.Key)
		paramVal := extractLitString(kv.Value)

		switch paramKey {
		case "statement_timeout":
			status.HasStatementTimeout = true
			if isZeroValue(paramVal) {
				status.HasZeroTimeout = true
				status.ZeroTimeoutParam = "statement_timeout"
			}
		case "lock_timeout":
			status.HasLockTimeout = true
			if isZeroValue(paramVal) {
				status.HasZeroTimeout = true
				status.ZeroTimeoutParam = "lock_timeout"
			}
		}
	}
}

// EvalBlockAssignments inspects statements in a block/function for assignments to configVar.
func EvalBlockAssignments(body *ast.BlockStmt, configVarName string) ConfigStatus {
	var status ConfigStatus
	if body == nil || configVarName == "" {
		return status
	}

	for _, stmt := range body.List {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				checkAssignment(lhs, i, s.Rhs, configVarName, &status)
			}
			for _, rhsExpr := range s.Rhs {
				if call, ok := rhsExpr.(*ast.CallExpr); ok {
					checkHelperCall(call, configVarName, &status)
				}
			}
		case *ast.ExprStmt:
			if call, ok := s.X.(*ast.CallExpr); ok {
				checkHelperCall(call, configVarName, &status)
			}
		case *ast.IfStmt:
			if s.Init != nil {
				if assign, ok := s.Init.(*ast.AssignStmt); ok {
					for i, lhs := range assign.Lhs {
						checkAssignment(lhs, i, assign.Rhs, configVarName, &status)
					}
					for _, rhsExpr := range assign.Rhs {
						if call, ok := rhsExpr.(*ast.CallExpr); ok {
							checkHelperCall(call, configVarName, &status)
						}
					}
				}
			}
			if call, ok := s.Cond.(*ast.CallExpr); ok {
				checkHelperCall(call, configVarName, &status)
			}
			if s.Body != nil {
				subStatus := EvalBlockAssignments(s.Body, configVarName)
				mergeStatus(&status, subStatus)
			}
		}
	}

	return status
}

func checkAssignment(lhs ast.Expr, idx int, rhs []ast.Expr, varName string, status *ConfigStatus) {
	lhsStr := exprToString(lhs)
	if !strings.HasPrefix(lhsStr, varName+".") && !strings.HasPrefix(lhsStr, varName+"[") {
		return
	}

	if strings.Contains(lhsStr, "MaxConnIdleTime") {
		status.HasMaxConnIdleTime = true
	}
	if strings.Contains(lhsStr, "MaxConnLifetime") {
		status.HasMaxConnLifetime = true
	}

	var rhsVal string
	if idx < len(rhs) {
		rhsVal = extractLitString(rhs[idx])
	}

	if strings.Contains(lhsStr, "statement_timeout") {
		status.HasStatementTimeout = true
		if isZeroValue(rhsVal) {
			status.HasZeroTimeout = true
			status.ZeroTimeoutParam = "statement_timeout"
		}
	}
	if strings.Contains(lhsStr, "lock_timeout") {
		status.HasLockTimeout = true
		if isZeroValue(rhsVal) {
			status.HasZeroTimeout = true
			status.ZeroTimeoutParam = "lock_timeout"
		}
	}

	if idx < len(rhs) {
		evalRuntimeParamsExpr(rhs[idx], status)
	}
}

func checkHelperCall(call *ast.CallExpr, varName string, status *ConfigStatus) {
	for _, arg := range call.Args {
		if id, ok := arg.(*ast.Ident); ok && id.Name == varName {
			fnName := exprToString(call.Fun)
			if strings.Contains(fnName, "configurePostgresPool") || strings.Contains(fnName, "Configure") {
				status.HasStatementTimeout = true
				status.HasLockTimeout = true
				status.HasMaxConnIdleTime = true
				status.HasMaxConnLifetime = true
			}
			if strings.Contains(fnName, "setRuntimeParam") && len(call.Args) >= 2 {
				paramName := extractLitString(call.Args[1])
				if paramName == "statement_timeout" {
					status.HasStatementTimeout = true
				}
				if paramName == "lock_timeout" {
					status.HasLockTimeout = true
				}
			}
		}
	}
}

func mergeStatus(dst *ConfigStatus, src ConfigStatus) {
	dst.HasStatementTimeout = dst.HasStatementTimeout || src.HasStatementTimeout
	dst.HasLockTimeout = dst.HasLockTimeout || src.HasLockTimeout
	dst.HasMaxConnIdleTime = dst.HasMaxConnIdleTime || src.HasMaxConnIdleTime
	dst.HasMaxConnLifetime = dst.HasMaxConnLifetime || src.HasMaxConnLifetime
	if src.HasZeroTimeout {
		dst.HasZeroTimeout = true
		dst.ZeroTimeoutParam = src.ZeroTimeoutParam
	}
}

func extractLitString(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return strings.Trim(lit.Value, "`\"")
	}
	return ""
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.IndexExpr:
		idxStr := ""
		if lit, ok := e.Index.(*ast.BasicLit); ok {
			idxStr = strings.Trim(lit.Value, "`\"")
		}
		return exprToString(e.X) + "[" + idxStr + "]"
	default:
		return ""
	}
}
