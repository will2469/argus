// Package a20_param_limit verifies chunking guardrails and slice bounds for batch database operations.
package a20_param_limit

import (
	"go/ast"
	"go/token"
	"strconv"
)

const maxSafeBatchSize = 1000

// IsChunkedLoop checks if a for loop implements safe chunking (e.g. i += chunkSize).
func IsChunkedLoop(forStmt *ast.ForStmt) bool {
	if forStmt == nil || forStmt.Post == nil {
		return false
	}

	// Pattern 1: i += chunkSize
	if assign, ok := forStmt.Post.(*ast.AssignStmt); ok {
		if assign.Tok == token.ADD_ASSIGN && len(assign.Rhs) > 0 {
			return isSafeChunkSizeExpr(assign.Rhs[0])
		}
		if assign.Tok == token.ASSIGN && len(assign.Rhs) > 0 {
			if bin, ok := assign.Rhs[0].(*ast.BinaryExpr); ok && bin.Op == token.ADD {
				return isSafeChunkSizeExpr(bin.Y) || isSafeChunkSizeExpr(bin.X)
			}
		}
	}

	return false
}

func isSafeChunkSizeExpr(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.INT {
		val, err := strconv.Atoi(lit.Value)
		if err == nil && val <= maxSafeBatchSize {
			return true
		}
	}
	if id, ok := expr.(*ast.Ident); ok {
		switch id.Name {
		case "chunkSize", "batchSize", "chunk", "step", "pageSize", "limit", "maxBatch":
			return true
		}
	}
	return false
}

// HasEnclosingChunkLoop checks if the call or node is enclosed in a chunking loop within the function.
func HasEnclosingChunkLoop(fn ast.Node, target ast.Node) bool {
	if fn == nil || target == nil {
		return false
	}

	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		if forStmt, ok := n.(*ast.ForStmt); ok {
			if IsChunkedLoop(forStmt) {
				if forStmt.Pos() <= target.Pos() && target.End() <= forStmt.End() {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}
