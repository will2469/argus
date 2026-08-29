// Package a23_tx_timeout inspects AST nodes for RuntimeParams["transaction_timeout"] configuration.
package a23_tx_timeout

import (
	"go/ast"
	"go/token"
	"strings"
)

// CheckRuntimeParamsMap inspects a map composite literal for transaction_timeout.
func CheckRuntimeParamsMap(compLit *ast.CompositeLit) (hasTxTimeout bool, isZero bool) {
	if compLit == nil {
		return false, false
	}

	for _, elt := range compLit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			key := extractStringLit(kv.Key)
			if key == "transaction_timeout" {
				val := extractStringLit(kv.Value)
				if val == "0" || val == "0s" || val == "0ms" {
					return true, true
				}
				return true, false
			}
		}
	}

	return false, false
}

// InspectFileForTxTimeout scans an AST file to determine if transaction_timeout is set via map index or helper.
func InspectFileForTxTimeout(file *ast.File) (hasConfig bool, isZero bool) {
	if file == nil {
		return false, false
	}

	found := false
	zero := false

	ast.Inspect(file, func(n ast.Node) bool {
		// Pattern 1: cfg.ConnConfig.RuntimeParams["transaction_timeout"] = "30000"
		if assign, ok := n.(*ast.AssignStmt); ok {
			for i, lhs := range assign.Lhs {
				if idxExpr, ok := lhs.(*ast.IndexExpr); ok {
					key := extractStringLit(idxExpr.Index)
					if key == "transaction_timeout" {
						found = true
						if i < len(assign.Rhs) {
							val := extractStringLit(assign.Rhs[i])
							if val == "0" || val == "0s" || val == "0ms" {
								zero = true
							}
						}
						return false
					}
				}
			}
		}

		// Pattern 2: setRuntimeParamDefault(config, "transaction_timeout", timeout)
		if call, ok := n.(*ast.CallExpr); ok {
			for _, arg := range call.Args {
				if str := extractStringLit(arg); str == "transaction_timeout" {
					found = true
					return false
				}
			}
		}

		return true
	})

	return found, zero
}

func extractStringLit(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		val := lit.Value
		if len(val) >= 2 && ((val[0] == '`' && val[len(val)-1] == '`') ||
			(val[0] == '"' && val[len(val)-1] == '"')) {
			return strings.TrimSpace(val[1 : len(val)-1])
		}
	}
	return ""
}
