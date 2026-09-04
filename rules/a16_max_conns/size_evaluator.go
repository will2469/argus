// Package a16_max_conns evaluates pool sizing thresholds for DSN strings and AST expressions.
package a16_max_conns

import (
	"fmt"
	"go/ast"
	"net/url"
	"strconv"
	"strings"
)

// DefaultMaxSafeConns is the recommended upper limit for direct connections per application pod.
const DefaultMaxSafeConns int32 = 100

// ConnsEvaluation represents the result of sizing evaluation.
type ConnsEvaluation struct {
	Configured bool
	Valid      bool
	Value      int32
	Message    string
}

// EvaluateDSN inspects a DSN URL or key-value string for pool_max_conns.
func EvaluateDSN(dsn string, maxSafe int32) ConnsEvaluation {
	if maxSafe <= 0 {
		maxSafe = DefaultMaxSafeConns
	}

	trimmed := strings.TrimSpace(dsn)
	valStr := ""

	if strings.Contains(trimmed, "://") {
		u, err := url.Parse(trimmed)
		if err == nil {
			valStr = u.Query().Get("pool_max_conns")
		}
	}

	if valStr == "" {
		for _, part := range strings.Fields(trimmed) {
			if strings.HasPrefix(part, "pool_max_conns=") {
				valStr = strings.TrimPrefix(part, "pool_max_conns=")
				break
			}
		}
	}

	if valStr == "" {
		return ConnsEvaluation{
			Configured: false,
			Message:    "Missing explicit pool_max_conns in DSN; default (4 * NumCPU) may cause connection exhaustion under burst",
		}
	}

	parsed64, err := strconv.ParseInt(valStr, 10, 32)
	if err != nil || parsed64 <= 0 {
		return ConnsEvaluation{
			Configured: true,
			Valid:      false,
			Message:    "pool_max_conns in DSN must be a positive integer greater than zero",
		}
	}
	parsed := int32(parsed64)

	if parsed > maxSafe {
		return ConnsEvaluation{
			Configured: true,
			Valid:      false,
			Value:      parsed,
			Message:    fmt.Sprintf("pool_max_conns (%s) in DSN exceeds safe direct connection threshold (%d); route through PgBouncer or reduce pool size", valStr, maxSafe),
		}
	}

	return ConnsEvaluation{
		Configured: true,
		Valid:      true,
		Value:      parsed,
	}
}

// EvaluateExpr inspects an AST expression assigned to MaxConns without file context.
func EvaluateExpr(expr ast.Expr, maxSafe int32) ConnsEvaluation {
	return EvaluateExprWithFile(nil, expr, maxSafe)
}

// EvaluateExprWithFile inspects an AST expression assigned to MaxConns with file context.
func EvaluateExprWithFile(file *ast.File, expr ast.Expr, maxSafe int32) ConnsEvaluation {
	if maxSafe <= 0 {
		maxSafe = DefaultMaxSafeConns
	}

	if expr == nil {
		return ConnsEvaluation{
			Configured: false,
			Valid:      false,
			Message:    "pgxpool.Config missing explicit MaxConns; set explicit value to prevent connection exhaustion",
		}
	}

	// 1. Statically evaluate constant integer expressions (literals, unary signed, arithmetic)
	if val, ok := evaluateConstInt(expr); ok {
		if val <= 0 {
			return ConnsEvaluation{
				Configured: true,
				Valid:      false,
				Value:      val,
				Message:    "MaxConns cannot be zero or negative; specify a valid positive connection limit",
			}
		}
		if val > maxSafe {
			return ConnsEvaluation{
				Configured: true,
				Valid:      false,
				Value:      val,
				Message:    fmt.Sprintf("MaxConns (%d) exceeds safe direct connection limit (%d) per pod; route via PgBouncer or reduce pool bounds", val, maxSafe),
			}
		}
		return ConnsEvaluation{
			Configured: true,
			Valid:      true,
			Value:      val,
		}
	}

	// 2. Function calls (e.g. ResolveMaxConns()): inspect return values semantically
	if call, ok := expr.(*ast.CallExpr); ok && file != nil {
		if eval, handled := inspectResolverCall(file, call, maxSafe); handled {
			return eval
		}
	}

	// 3. Identifiers: attempt to resolve constant declaration in the file
	if id, ok := expr.(*ast.Ident); ok && file != nil {
		if val, ok := findConstIdentValue(file, id.Name); ok {
			if val <= 0 {
				return ConnsEvaluation{
					Configured: true,
					Valid:      false,
					Value:      val,
					Message:    "MaxConns cannot be zero or negative; specify a valid positive connection limit",
				}
			}
			if val > maxSafe {
				return ConnsEvaluation{
					Configured: true,
					Valid:      false,
					Value:      val,
					Message:    fmt.Sprintf("MaxConns (%d) exceeds safe direct connection limit (%d) per pod; route via PgBouncer or reduce pool bounds", val, maxSafe),
				}
			}
			return ConnsEvaluation{
				Configured: true,
				Valid:      true,
				Value:      val,
			}
		}
	}

	// 4. Unresolvable / unverified dynamic expressions fail closed
	exprStr := ""
	if id, ok := expr.(*ast.Ident); ok {
		exprStr = id.Name
	} else if call, ok := expr.(*ast.CallExpr); ok {
		if id, ok := call.Fun.(*ast.Ident); ok {
			exprStr = id.Name + "()"
		}
	}
	if exprStr != "" {
		exprStr = " (" + exprStr + ")"
	}

	return ConnsEvaluation{
		Configured: true,
		Valid:      false,
		Message:    fmt.Sprintf("MaxConns%s assigned unverified dynamic value; ensure explicit, statically provable bound (<= %d)", exprStr, maxSafe),
	}
}

func findConstIdentValue(file *ast.File, name string) (int32, bool) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			valSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, ident := range valSpec.Names {
				if ident.Name == name && i < len(valSpec.Values) {
					return evaluateConstInt(valSpec.Values[i])
				}
			}
		}
	}
	return 0, false
}
