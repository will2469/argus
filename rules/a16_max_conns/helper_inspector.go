// Package a16_max_conns inspects helper functions and resolvers for MaxConns configuration.
package a16_max_conns

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"math"
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

	var returnVals []int64
	hasReturn := false
	hasDynamicReturn := false

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		ret, ok := n.(*ast.ReturnStmt)
		if !ok || len(ret.Results) == 0 {
			return true
		}
		hasReturn = true
		for _, res := range ret.Results {
			if val, ok := evaluateConstInt64(res); ok {
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
				Value:      int32(val),
				Message:    "MaxConns cannot be zero or negative; specify a valid positive connection limit",
			}, true
		}
		if val > int64(maxSafe) {
			val32 := int32(math.MaxInt32)
			if val <= math.MaxInt32 {
				val32 = int32(val)
			}
			return ConnsEvaluation{
				Configured: true,
				Valid:      false,
				Value:      val32,
				Message:    fmt.Sprintf("MaxConns (%d) exceeds safe direct connection limit (%d) per pod; route via PgBouncer or reduce pool bounds", val, maxSafe),
			}, true
		}
	}

	last := returnVals[len(returnVals)-1]
	last32 := int32(math.MaxInt32)
	if last <= math.MaxInt32 {
		last32 = int32(last)
	}
	return ConnsEvaluation{
		Configured: true,
		Valid:      true,
		Value:      last32,
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

	finalState, _ := EvalBlockFlow(file, fn.Body, token.NoPos, paramName, maxSafe, ConfigFlowState{})
	return finalState.Configured, finalState.Valid
}

// evaluateConstValue recursively evaluates an AST expression using go/constant arbitrary-precision arithmetic.
func evaluateConstValue(expr ast.Expr) constant.Value {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		return constant.MakeFromLiteral(e.Value, e.Kind, 0)
	case *ast.UnaryExpr:
		val := evaluateConstValue(e.X)
		if val != nil {
			return constant.UnaryOp(e.Op, val, 0)
		}
	case *ast.BinaryExpr:
		left := evaluateConstValue(e.X)
		right := evaluateConstValue(e.Y)
		if left != nil && right != nil {
			if e.Op == token.QUO && constant.Sign(right) == 0 {
				return nil // divide by zero
			}
			return constant.BinaryOp(left, e.Op, right)
		}
	case *ast.ParenExpr:
		return evaluateConstValue(e.X)
	}
	return nil
}

// evaluateConstInt64 statically evaluates an AST integer expression to int64 with overflow protection.
func evaluateConstInt64(expr ast.Expr) (int64, bool) {
	val := evaluateConstValue(expr)
	if val == nil || val.Kind() != constant.Int {
		return 0, false
	}
	i64, exact := constant.Int64Val(val)
	if !exact {
		if constant.Sign(val) > 0 {
			return math.MaxInt64, true
		}
		return math.MinInt64, true
	}
	return i64, true
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
