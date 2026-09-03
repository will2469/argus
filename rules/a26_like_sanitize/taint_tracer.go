// Package a26_like_sanitize traces Go argument expressions to verify wildcard sanitization.
package a26_like_sanitize

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// VarState represents the sanitization state of a variable during dataflow analysis.
type VarState int

const (
	StateUnsanitized VarState = iota
	StateSanitized
)

// IsArgumentSanitized traces an AST expression to determine if it is properly sanitized against SQL wildcard injection.
func IsArgumentSanitized(expr ast.Expr, body *ast.BlockStmt, reg *SanitizerRegistry, pass *analysis.Pass) bool {
	if expr == nil {
		return true
	}
	if reg == nil {
		reg = NewSanitizerRegistry(pass, nil, nil, nil)
	}

	switch e := expr.(type) {
	case *ast.BasicLit:
		// Static constant strings: check for pathological wildcards (CWE-400)
		if isPatho, _ := reg.IsPathologicalLiteral(e); isPatho {
			return false
		}
		return true

	case *ast.CallExpr:
		return reg.IsSanitizerCall(pass, e)

	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return isBinaryConcatSanitized(e, body, reg, pass)
		}

	case *ast.Ident:
		return isIdentSanitized(e, body, reg, pass)
	}

	return false
}

func isBinaryConcatSanitized(bin *ast.BinaryExpr, body *ast.BlockStmt, reg *SanitizerRegistry, pass *analysis.Pass) bool {
	leftSafe := isConcatPartSanitized(bin.X, body, reg, pass)
	rightSafe := isConcatPartSanitized(bin.Y, body, reg, pass)
	return leftSafe && rightSafe
}

func isConcatPartSanitized(expr ast.Expr, body *ast.BlockStmt, reg *SanitizerRegistry, pass *analysis.Pass) bool {
	if _, ok := expr.(*ast.BasicLit); ok {
		return true // Literal prefix or suffix attached to a pattern
	}
	return IsArgumentSanitized(expr, body, reg, pass)
}

func isExprSanitizedWithEnv(expr ast.Expr, env map[string]VarState, reg *SanitizerRegistry, pass *analysis.Pass) bool {
	if expr == nil {
		return true
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		return true // Literal prefix or suffix attached to variable
	case *ast.CallExpr:
		return reg.IsSanitizerCall(pass, e)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return isExprSanitizedWithEnv(e.X, env, reg, pass) && isExprSanitizedWithEnv(e.Y, env, reg, pass)
		}
	case *ast.Ident:
		return env[e.Name] == StateSanitized
	}
	return false
}

func evalBlock(block *ast.BlockStmt, targetPos token.Pos, env map[string]VarState, reg *SanitizerRegistry, pass *analysis.Pass) (map[string]VarState, bool) {
	if block == nil {
		return env, false
	}

	currentEnv := make(map[string]VarState)
	for k, v := range env {
		currentEnv[k] = v
	}

	for _, stmt := range block.List {
		if targetPos != token.NoPos && targetPos < stmt.Pos() {
			return currentEnv, true
		}

		if targetPos != token.NoPos && targetPos >= stmt.Pos() && targetPos <= stmt.End() {
			switch s := stmt.(type) {
			case *ast.IfStmt:
				return evalIf(s, targetPos, currentEnv, reg, pass)
			case *ast.BlockStmt:
				return evalBlock(s, targetPos, currentEnv, reg, pass)
			case *ast.ExprStmt:
				return currentEnv, true
			default:
				return currentEnv, true
			}
		}

		switch s := stmt.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					if i < len(s.Rhs) {
						if isExprSanitizedWithEnv(s.Rhs[i], currentEnv, reg, pass) {
							currentEnv[id.Name] = StateSanitized
						} else {
							currentEnv[id.Name] = StateUnsanitized
						}
					}
				}
			}
		case *ast.IfStmt:
			currentEnv = evalIfJoin(s, currentEnv, reg, pass)
		case *ast.BlockStmt:
			currentEnv, _ = evalBlock(s, token.NoPos, currentEnv, reg, pass)
		}
	}

	return currentEnv, false
}

func evalIf(s *ast.IfStmt, targetPos token.Pos, env map[string]VarState, reg *SanitizerRegistry, pass *analysis.Pass) (map[string]VarState, bool) {
	if s.Init != nil && targetPos >= s.Init.Pos() && targetPos <= s.Init.End() {
		return env, true
	}
	if targetPos >= s.Cond.Pos() && targetPos <= s.Cond.End() {
		return env, true
	}
	if s.Body != nil && targetPos >= s.Body.Pos() && targetPos <= s.Body.End() {
		return evalBlock(s.Body, targetPos, env, reg, pass)
	}
	if s.Else != nil && targetPos >= s.Else.Pos() && targetPos <= s.Else.End() {
		switch el := s.Else.(type) {
		case *ast.BlockStmt:
			return evalBlock(el, targetPos, env, reg, pass)
		case *ast.IfStmt:
			return evalIf(el, targetPos, env, reg, pass)
		}
	}
	return env, true
}

func evalIfJoin(s *ast.IfStmt, env map[string]VarState, reg *SanitizerRegistry, pass *analysis.Pass) map[string]VarState {
	thenEnv, _ := evalBlock(s.Body, token.NoPos, env, reg, pass)

	var elseEnv map[string]VarState
	if s.Else != nil {
		switch el := s.Else.(type) {
		case *ast.BlockStmt:
			elseEnv, _ = evalBlock(el, token.NoPos, env, reg, pass)
		case *ast.IfStmt:
			elseEnv = evalIfJoin(el, env, reg, pass)
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

func isIdentSanitized(ident *ast.Ident, body *ast.BlockStmt, reg *SanitizerRegistry, pass *analysis.Pass) bool {
	if ident == nil || body == nil {
		return false
	}
	envAtTarget, _ := evalBlock(body, ident.Pos(), nil, reg, pass)
	return envAtTarget[ident.Name] == StateSanitized
}
