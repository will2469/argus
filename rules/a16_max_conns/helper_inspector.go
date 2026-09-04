// Package a16_max_conns inspects helper functions and resolvers for MaxConns configuration.
package a16_max_conns

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
)

// findFuncDecl locates a FuncDecl in the file by function expression (ident or selector).
func findFuncDecl(file *ast.File, fnExpr ast.Expr) *ast.FuncDecl {
	if file == nil || fnExpr == nil {
		return nil
	}
	name := ""
	switch e := fnExpr.(type) {
	case *ast.Ident:
		name = e.Name
	case *ast.SelectorExpr:
		name = e.Sel.Name
	}
	if name == "" {
		return nil
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name != nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// inspectResolverCall checks if a call expression (e.g. ResolveMaxConns()) statically returns a bounded constant.
func inspectResolverCall(file *ast.File, call *ast.CallExpr, maxSafe int32) (ConnsEvaluation, bool) {
	fn := findFuncDecl(file, call.Fun)
	if fn == nil || fn.Body == nil {
		return ConnsEvaluation{}, false
	}

	var returnVals []int32
	hasReturn := false
	hasDynamicReturn := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			return true
		}
		hasReturn = true
		for _, res := range ret.Results {
			if val, ok := evaluateConstInt(res); ok {
				returnVals = append(returnVals, val)
			} else {
				hasDynamicReturn = true
			}
		}
		return true
	})

	if !hasReturn || hasDynamicReturn || len(returnVals) == 0 {
		fnName := ""
		if id, ok := call.Fun.(*ast.Ident); ok {
			fnName = id.Name
		}
		return ConnsEvaluation{
			Configured: true,
			Valid:      false,
			Message:    fmt.Sprintf("MaxConns resolver %s returns unverified dynamic value; ensure explicit bounded limit (<= %d)", fnName, maxSafe),
		}, true
	}

	for _, val := range returnVals {
		if val <= 0 {
			return ConnsEvaluation{
				Configured: true,
				Valid:      false,
				Value:      val,
				Message:    "MaxConns cannot be zero or negative; specify a valid positive connection limit",
			}, true
		}
		if val > maxSafe {
			return ConnsEvaluation{
				Configured: true,
				Valid:      false,
				Value:      val,
				Message:    fmt.Sprintf("MaxConns (%d) exceeds safe direct connection limit (%d) per pod; route via PgBouncer or reduce pool bounds", val, maxSafe),
			}, true
		}
	}

	return ConnsEvaluation{
		Configured: true,
		Valid:      true,
		Value:      returnVals[len(returnVals)-1],
	}, true
}

// inspectConfigMutatorHelper verifies whether a helper function (e.g. ApplyPoolLimits(cfg))
// explicitly assigns a valid, bounded MaxConns on the passed config parameter.
func inspectConfigMutatorHelper(file *ast.File, call *ast.CallExpr, cfgArgIndex int, maxSafe int32) (bool, bool) {
	fn := findFuncDecl(file, call.Fun)
	if fn == nil || fn.Body == nil || fn.Type == nil || fn.Type.Params == nil {
		return false, false
	}

	var paramName string
	currIdx := 0
	for _, field := range fn.Type.Params.List {
		for _, name := range field.Names {
			if currIdx == cfgArgIndex {
				paramName = name.Name
				break
			}
			currIdx++
		}
		if paramName != "" {
			break
		}
	}

	if paramName == "" {
		return false, false
	}

	foundMutation := false
	isValid := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range assign.Lhs {
			sel, ok := lhs.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Name != paramName || sel.Sel.Name != "MaxConns" {
				continue
			}
			foundMutation = true
			if i < len(assign.Rhs) {
				eval := EvaluateExprWithFile(file, assign.Rhs[i], maxSafe)
				if eval.Valid {
					isValid = true
				}
			}
		}
		return true
	})

	return foundMutation, isValid
}

// evaluateConstInt statically evaluates an AST integer expression (literals, unary signed, arithmetic).
func evaluateConstInt(expr ast.Expr) (int32, bool) {
	if expr == nil {
		return 0, false
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.INT {
			v, err := strconv.ParseInt(e.Value, 10, 32)
			if err == nil {
				return int32(v), true
			}
		}
	case *ast.UnaryExpr:
		if inner, ok := evaluateConstInt(e.X); ok {
			switch e.Op {
			case token.SUB:
				return -inner, true
			case token.ADD:
				return inner, true
			}
		}
	case *ast.BinaryExpr:
		left, ok1 := evaluateConstInt(e.X)
		right, ok2 := evaluateConstInt(e.Y)
		if ok1 && ok2 {
			switch e.Op {
			case token.ADD:
				return left + right, true
			case token.SUB:
				return left - right, true
			case token.MUL:
				return left * right, true
			case token.QUO:
				if right != 0 {
					return left / right, true
				}
			}
		}
	case *ast.ParenExpr:
		return evaluateConstInt(e.X)
	}
	return 0, false
}

func evaluateCompositeLit(file *ast.File, composite *ast.CompositeLit, maxSafe int32) ConfigEvaluationResult {
	var maxConnsExpr ast.Expr
	var reportNode ast.Node = composite

	for _, elt := range composite.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok || ident.Name != "MaxConns" {
			continue
		}
		maxConnsExpr = kv.Value
		reportNode = kv
	}

	if maxConnsExpr == nil {
		return ConfigEvaluationResult{
			HasMaxConns: false,
			Valid:       false,
			Message:     "pgxpool.Config literal missing MaxConns; set explicit bound to avoid default 4 * NumCPU exhaustion",
			ReportNode:  composite,
		}
	}

	eval := EvaluateExprWithFile(file, maxConnsExpr, maxSafe)
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

