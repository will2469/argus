// Package a12_timeout_config tracks pgxpool.Config initialization and composite AST structures.
package a12_timeout_config

import (
	"go/ast"
	"go/token"
	"strings"
)

// ConfigStatus tracks presence of required timeout configurations.
type ConfigStatus struct {
	HasStatementTimeout  bool
	HasLockTimeout       bool
	HasIdleInTransaction bool
	HasMaxConnIdleTime   bool
	HasMaxConnLifetime   bool
	HasZeroTimeout       bool
	ZeroTimeoutParam     string
}

// EvalCompositeLit evaluates a pgxpool.Config composite literal.
func EvalCompositeLit(lit *ast.CompositeLit) ConfigStatus {
	var status ConfigStatus
	if lit == nil {
		return status
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

		applyRuntimeParamMutation(paramKey, paramVal, status)
	}
}

func evalConfigCallExpr(call *ast.CallExpr) ConfigStatus {
	var status ConfigStatus
	if call == nil {
		return status
	}
	fnName := exprToString(call.Fun)
	if strings.Contains(fnName, "ParseConfig") {
		if len(call.Args) > 0 {
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				dsn := strings.Trim(lit.Value, "`\"")
				res := CheckDSN(dsn)
				hasStatement := true
				hasLock := true
				hasIdle := true
				for _, m := range res.Missing {
					switch m {
					case "statement_timeout":
						hasStatement = false
					case "lock_timeout":
						hasLock = false
					case "idle_in_transaction_session_timeout", "idle_in_transaction":
						hasIdle = false
					}
				}
				status.HasStatementTimeout = hasStatement
				status.HasLockTimeout = hasLock
				status.HasIdleInTransaction = hasIdle
				if len(res.Zero) > 0 {
					status.HasZeroTimeout = true
					status.ZeroTimeoutParam = res.Zero[0]
				}
			}
		}
		return status
	}
	if strings.Contains(fnName, "goodConfig") || strings.Contains(fnName, "DefaultConfig") || strings.Contains(fnName, "configurePostgresPool") {
		status.HasStatementTimeout = true
		status.HasLockTimeout = true
		status.HasIdleInTransaction = true
		status.HasMaxConnIdleTime = true
		status.HasMaxConnLifetime = true
	}
	return status
}

func applyRuntimeParamMutation(key, val string, status *ConfigStatus) {
	switch key {
	case "statement_timeout":
		status.HasStatementTimeout = true
		if isZeroValue(val) {
			status.HasZeroTimeout = true
			status.ZeroTimeoutParam = "statement_timeout"
		} else if status.ZeroTimeoutParam == "statement_timeout" {
			status.HasZeroTimeout = false
			status.ZeroTimeoutParam = ""
		}
	case "lock_timeout":
		status.HasLockTimeout = true
		if isZeroValue(val) {
			status.HasZeroTimeout = true
			status.ZeroTimeoutParam = "lock_timeout"
		} else if status.ZeroTimeoutParam == "lock_timeout" {
			status.HasZeroTimeout = false
			status.ZeroTimeoutParam = ""
		}
	case "idle_in_transaction_session_timeout", "idle_in_transaction":
		status.HasIdleInTransaction = true
		if isZeroValue(val) {
			status.HasZeroTimeout = true
			status.ZeroTimeoutParam = "idle_in_transaction_session_timeout"
		} else if status.ZeroTimeoutParam == "idle_in_transaction_session_timeout" {
			status.HasZeroTimeout = false
			status.ZeroTimeoutParam = ""
		}
	}
}

func getRootIdent(expr ast.Expr) *ast.Ident {
	switch e := expr.(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return getRootIdent(e.X)
	case *ast.IndexExpr:
		return getRootIdent(e.X)
	case *ast.UnaryExpr:
		return getRootIdent(e.X)
	case *ast.ParenExpr:
		return getRootIdent(e.X)
	default:
		return nil
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
