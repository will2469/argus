package a16_max_conns

import (
	"go/ast"
	"strings"
)

// ConfigEvaluationResult holds the evaluated MaxConns status of a pgxpool.Config.
type ConfigEvaluationResult struct {
	HasMaxConns bool
	Valid       bool
	Message     string
	ReportNode  ast.Node
}

// TrackConfig evaluates the MaxConns configuration for a pgxpool.Config expression.
func TrackConfig(cfgExpr ast.Expr, file *ast.File, maxSafe int32) ConfigEvaluationResult {
	// 1. Direct CompositeLit
	if composite, ok := unwrapCompositeLit(cfgExpr); ok {
		return evaluateCompositeLit(composite, maxSafe)
	}

	// 2. Variable reference: track assignments in enclosing function
	varName := ""
	if ident, ok := cfgExpr.(*ast.Ident); ok {
		varName = ident.Name
	}

	if varName == "" || file == nil {
		return ConfigEvaluationResult{
			HasMaxConns: false,
			Valid:       false,
			Message:     "pgxpool.Config missing explicit MaxConns; relying on default (4 * NumCPU) risks connection starvation",
			ReportNode:  cfgExpr,
		}
	}

	fn := findEnclosingFunc(file, cfgExpr)
	if fn == nil || fn.Body == nil {
		return ConfigEvaluationResult{
			HasMaxConns: false,
			Valid:       false,
			Message:     "pgxpool.Config missing explicit MaxConns; relying on default (4 * NumCPU) risks connection starvation",
			ReportNode:  cfgExpr,
		}
	}

	// Exempt factory functions whose purpose is to return default/unfilled configs
	if fn.Name != nil && (strings.HasPrefix(fn.Name.Name, "ParseConfig") || strings.HasPrefix(fn.Name.Name, "DefaultConfig")) {
		return ConfigEvaluationResult{HasMaxConns: true, Valid: true}
	}

	return scanFuncBodyForMaxConns(fn.Body, varName, cfgExpr, maxSafe)
}

func scanFuncBodyForMaxConns(body *ast.BlockStmt, varName string, cfgExpr ast.Expr, maxSafe int32) ConfigEvaluationResult {
	var maxConnsAssign *ast.AssignStmt
	var assignedExpr ast.Expr
	hasHelperConfig := false

	ast.Inspect(body, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				ident, ok := sel.X.(*ast.Ident)
				if !ok || ident.Name != varName {
					continue
				}

				if sel.Sel.Name == "MaxConns" {
					maxConnsAssign = assign
					if len(assign.Rhs) > 0 {
						assignedExpr = assign.Rhs[0]
					}
				}
			}

			// Check if config variable is passed to a pool configuration helper
			for _, rhs := range assign.Rhs {
				if call, ok := rhs.(*ast.CallExpr); ok {
					for _, arg := range call.Args {
						if id, ok := arg.(*ast.Ident); ok && id.Name == varName {
							if isConfigHelper(call.Fun) {
								hasHelperConfig = true
							}
						}
					}
				}
			}
		}

		if exprStmt, ok := n.(*ast.ExprStmt); ok {
			if call, ok := exprStmt.X.(*ast.CallExpr); ok {
				for _, arg := range call.Args {
					if id, ok := arg.(*ast.Ident); ok && id.Name == varName {
						if isConfigHelper(call.Fun) {
							hasHelperConfig = true
						}
					}
				}
			}
		}

		return true
	})

	if hasHelperConfig {
		return ConfigEvaluationResult{HasMaxConns: true, Valid: true}
	}

	if maxConnsAssign == nil {
		return ConfigEvaluationResult{
			HasMaxConns: false,
			Valid:       false,
			Message:     "pgxpool.Config missing explicit MaxConns; set explicit value (e.g. 10-25 per pod) to prevent connection exhaustion",
			ReportNode:  cfgExpr,
		}
	}

	eval := EvaluateExpr(assignedExpr, maxSafe)
	if !eval.Valid {
		return ConfigEvaluationResult{
			HasMaxConns: true,
			Valid:       false,
			Message:     eval.Message,
			ReportNode:  maxConnsAssign,
		}
	}

	return ConfigEvaluationResult{HasMaxConns: true, Valid: true}
}

func evaluateCompositeLit(composite *ast.CompositeLit, maxSafe int32) ConfigEvaluationResult {
	var maxConnsExpr ast.Expr
	var reportNode ast.Node = composite

	for _, elt := range composite.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		if ident.Name == "MaxConns" {
			maxConnsExpr = kv.Value
			reportNode = kv
		}
	}

	if maxConnsExpr == nil {
		return ConfigEvaluationResult{
			HasMaxConns: false,
			Valid:       false,
			Message:     "pgxpool.Config literal missing MaxConns; set explicit bound to avoid default 4 * NumCPU exhaustion",
			ReportNode:  composite,
		}
	}

	eval := EvaluateExpr(maxConnsExpr, maxSafe)
	if !eval.Valid {
		return ConfigEvaluationResult{
			HasMaxConns: true,
			Valid:       false,
			Message:     eval.Message,
			ReportNode:  reportNode,
		}
	}

	return ConfigEvaluationResult{HasMaxConns: true, Valid: true}
}

func unwrapCompositeLit(expr ast.Expr) (*ast.CompositeLit, bool) {
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}
	lit, ok := expr.(*ast.CompositeLit)
	return lit, ok
}

func findEnclosingFunc(file *ast.File, target ast.Node) *ast.FuncDecl {
	var enclosing *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Pos() <= target.Pos() && target.End() <= fn.End() {
				enclosing = fn
			}
		}
		return true
	})
	return enclosing
}

func isConfigHelper(fnExpr ast.Expr) bool {
	name := ""
	switch e := fnExpr.(type) {
	case *ast.Ident:
		name = e.Name
	case *ast.SelectorExpr:
		name = e.Sel.Name
	}
	lower := strings.ToLower(name)
	if lower == "newwithconfig" || lower == "new" || lower == "parseconfig" {
		return false
	}
	return strings.Contains(lower, "pool") || strings.Contains(lower, "config")
}
