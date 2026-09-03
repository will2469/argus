package a16_max_conns

import (
	"go/ast"
	"go/token"
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
			Message:    "pool_max_conns (" + valStr + ") in DSN exceeds safe direct connection threshold (100); route through PgBouncer or reduce pool size",
		}
	}

	return ConnsEvaluation{
		Configured: true,
		Valid:      true,
		Value:      parsed,
	}
}

// EvaluateExpr inspects an AST expression assigned to MaxConns.
func EvaluateExpr(expr ast.Expr, maxSafe int32) ConnsEvaluation {
	if maxSafe <= 0 {
		maxSafe = DefaultMaxSafeConns
	}

	multiplier := 1
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		switch unary.Op {
		case token.SUB:
			multiplier = -1
			expr = unary.X
		case token.ADD:
			expr = unary.X
		}
	}

	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		// Variable, function call (e.g. ResolveMaxConns()), or ENV fallback is treated as configured.
		return ConnsEvaluation{
			Configured: true,
			Valid:      true,
		}
	}

	val64, err := strconv.ParseInt(lit.Value, 10, 32)
	val := int32(val64) * int32(multiplier)
	if err != nil || val <= 0 {
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
			Message:    "MaxConns (" + lit.Value + ") exceeds safe direct connection limit (100) per pod; route via PgBouncer or reduce pool bounds",
		}
	}

	return ConnsEvaluation{
		Configured: true,
		Valid:      true,
		Value:      val,
	}
}
