// Package a26_like_sanitize traces Go argument expressions to verify wildcard sanitization.
package a26_like_sanitize

import (
	"go/ast"
	"go/token"
	"strings"
)

// VarState represents the sanitization state of a variable during dataflow analysis.
type VarState int

const (
	StateUnsanitized VarState = iota
	StateSanitized
)

// IsArgumentSanitized traces an AST expression to determine if it is properly sanitized against SQL wildcard injection.
func IsArgumentSanitized(expr ast.Expr, body *ast.BlockStmt) bool {
	if expr == nil {
		return true
	}

	switch e := expr.(type) {
	case *ast.BasicLit:
		// Static constant strings (e.g. "STATUS_%") are safe
		return true

	case *ast.CallExpr:
		return isSanitizerCall(e)

	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			// Concatenation: check non-literal components
			return isBinaryConcatSanitized(e, body)
		}

	case *ast.Ident:
		return isIdentSanitized(e, body)
	}

	return false
}

func isSanitizerCall(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}

	methodName := strings.ToLower(getCallMethodName(call.Fun))
	switch methodName {
	case "sanitizelikepattern", "sanitizelike", "sanitizelikewildcards", "sanitizewildcards",
		"formatlikecontains", "formatlikeprefix", "formatlikesuffix",
		"escapelikepattern", "escapelike", "escapelikewildcards", "escapelikestring",
		"quotelikepattern", "quotelike",
		"cleanlikepattern", "cleanlike":
		return true
	}

	return false
}

func isBinaryConcatSanitized(bin *ast.BinaryExpr, body *ast.BlockStmt) bool {
	leftSafe := IsArgumentSanitized(bin.X, body)
	rightSafe := IsArgumentSanitized(bin.Y, body)
	return leftSafe && rightSafe
}

func isExprSanitizedWithEnv(expr ast.Expr, env map[string]VarState) bool {
	if expr == nil {
		return true
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.CallExpr:
		return isSanitizerCall(e)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return isExprSanitizedWithEnv(e.X, env) && isExprSanitizedWithEnv(e.Y, env)
		}
	case *ast.Ident:
		return env[e.Name] == StateSanitized
	}
	return false
}

// evalBlock traces block statements linearly up to targetPos and returns the resulting environment state.
// If targetPos is encountered inside the block or a nested statement, returns the state at targetPos.
func evalBlock(block *ast.BlockStmt, targetPos token.Pos, env map[string]VarState) (map[string]VarState, bool) {
	if block == nil {
		return env, false
	}

	currentEnv := make(map[string]VarState)
	for k, v := range env {
		currentEnv[k] = v
	}

	for _, stmt := range block.List {
		// If the target position is strictly before this statement starts, we stop
		if targetPos != token.NoPos && targetPos < stmt.Pos() {
			return currentEnv, true
		}

		// Check if target is INSIDE this statement
		if targetPos != token.NoPos && targetPos >= stmt.Pos() && targetPos <= stmt.End() {
			switch s := stmt.(type) {
			case *ast.IfStmt:
				return evalIf(s, targetPos, currentEnv)
			case *ast.BlockStmt:
				return evalBlock(s, targetPos, currentEnv)
			case *ast.ExprStmt:
				return currentEnv, true
			default:
				return currentEnv, true
			}
		}

		// Statement executed completely before targetPos: update environment
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					if i < len(s.Rhs) {
						if isExprSanitizedWithEnv(s.Rhs[i], currentEnv) {
							currentEnv[id.Name] = StateSanitized
						} else {
							currentEnv[id.Name] = StateUnsanitized
						}
					}
				}
			}
		case *ast.IfStmt:
			currentEnv = evalIfJoin(s, currentEnv)
		case *ast.BlockStmt:
			currentEnv, _ = evalBlock(s, token.NoPos, currentEnv)
		}
	}

	return currentEnv, false
}

func evalIf(s *ast.IfStmt, targetPos token.Pos, env map[string]VarState) (map[string]VarState, bool) {
	if s.Init != nil && targetPos >= s.Init.Pos() && targetPos <= s.Init.End() {
		return env, true
	}
	if targetPos >= s.Cond.Pos() && targetPos <= s.Cond.End() {
		return env, true
	}
	if s.Body != nil && targetPos >= s.Body.Pos() && targetPos <= s.Body.End() {
		return evalBlock(s.Body, targetPos, env)
	}
	if s.Else != nil && targetPos >= s.Else.Pos() && targetPos <= s.Else.End() {
		switch el := s.Else.(type) {
		case *ast.BlockStmt:
			return evalBlock(el, targetPos, env)
		case *ast.IfStmt:
			return evalIf(el, targetPos, env)
		}
	}
	return env, true
}

func evalIfJoin(s *ast.IfStmt, env map[string]VarState) map[string]VarState {
	thenEnv, _ := evalBlock(s.Body, token.NoPos, env)

	var elseEnv map[string]VarState
	if s.Else != nil {
		switch el := s.Else.(type) {
		case *ast.BlockStmt:
			elseEnv, _ = evalBlock(el, token.NoPos, env)
		case *ast.IfStmt:
			elseEnv = evalIfJoin(el, env)
		default:
			elseEnv = env
		}
	} else {
		elseEnv = env
	}

	joined := make(map[string]VarState)
	for k := range env {
		if thenEnv[k] == StateSanitized && elseEnv[k] == StateSanitized {
			joined[k] = StateSanitized
		} else {
			joined[k] = StateUnsanitized
		}
	}
	for k := range thenEnv {
		if thenEnv[k] == StateSanitized && elseEnv[k] == StateSanitized {
			joined[k] = StateSanitized
		} else {
			joined[k] = StateUnsanitized
		}
	}

	return joined
}

func isIdentSanitized(ident *ast.Ident, body *ast.BlockStmt) bool {
	if ident == nil || body == nil {
		return false
	}
	envAtTarget, _ := evalBlock(body, ident.Pos(), nil)
	return envAtTarget[ident.Name] == StateSanitized
}

func getCallMethodName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}
