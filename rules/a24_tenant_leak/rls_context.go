// Package a24_tenant_leak inspects function AST scope for RLS session parameter setups.
package a24_tenant_leak

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"github.com/will2469/argus/shared/callsite"
)

func buildRLSRegex(tenantCol string) *regexp.Regexp {
	colPattern := "tenant_id|org_id"
	if tenantCol != "" && tenantCol != "tenant_id" && tenantCol != "org_id" {
		colPattern = "tenant_id|org_id|" + regexp.QuoteMeta(strings.ToLower(tenantCol))
	}
	rlsPattern := `(?i)\bSET\s+(?:LOCAL\s+)?app\.(?:` + colPattern + `)\b|\bset_config\s*\(\s*['"]app\.(?:` + colPattern + `)['"]`
	return regexp.MustCompile(rlsPattern)
}

func isRLSCall(call *ast.CallExpr, rlsRegex *regexp.Regexp) bool {
	if call == nil || rlsRegex == nil {
		return false
	}
	if sql, ok := callsite.ExtractQueryString(call); ok {
		return rlsRegex.MatchString(sql)
	}
	return false
}

func isTerminatingStmt(stmt ast.Stmt) bool {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ExprStmt:
		if call, ok := s.X.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "panic" || id.Name == "fatal") {
				return true
			}
		}
	case *ast.BlockStmt:
		if len(s.List) > 0 {
			return isTerminatingStmt(s.List[len(s.List)-1])
		}
	}
	return false
}

// IsRLSActiveAt determines if RLS is actively configured at targetPos via control-flow dominance.
func IsRLSActiveAt(body *ast.BlockStmt, targetPos token.Pos, tenantCol string) bool {
	if body == nil {
		return false
	}
	rlsRegex := buildRLSRegex(tenantCol)
	active, _ := evalBlockRLS(body, targetPos, false, rlsRegex)
	return active
}

func evalBlockRLS(block *ast.BlockStmt, targetPos token.Pos, rlsActive bool, rlsRegex *regexp.Regexp) (bool, bool) {
	if block == nil {
		return rlsActive, false
	}

	for _, stmt := range block.List {
		if targetPos != token.NoPos && targetPos < stmt.Pos() {
			return rlsActive, true
		}

		if targetPos != token.NoPos && targetPos >= stmt.Pos() && targetPos <= stmt.End() {
			switch s := stmt.(type) {
			case *ast.IfStmt:
				return evalIfRLS(s, targetPos, rlsActive, rlsRegex)
			case *ast.BlockStmt:
				return evalBlockRLS(s, targetPos, rlsActive, rlsRegex)
			default:
				return rlsActive, true
			}
		}

		switch s := stmt.(type) {
		case *ast.ExprStmt:
			if call, ok := s.X.(*ast.CallExpr); ok && isRLSCall(call, rlsRegex) {
				rlsActive = true
			}
		case *ast.AssignStmt:
			for _, rhs := range s.Rhs {
				if call, ok := rhs.(*ast.CallExpr); ok && isRLSCall(call, rlsRegex) {
					rlsActive = true
				}
			}
		case *ast.IfStmt:
			rlsActive = evalIfJoinRLS(s, rlsActive, rlsRegex)
		case *ast.BlockStmt:
			rlsActive, _ = evalBlockRLS(s, token.NoPos, rlsActive, rlsRegex)
		}
	}

	return rlsActive, false
}

func evalIfRLS(s *ast.IfStmt, targetPos token.Pos, rlsActive bool, rlsRegex *regexp.Regexp) (bool, bool) {
	if s.Init != nil && targetPos >= s.Init.Pos() && targetPos <= s.Init.End() {
		return rlsActive, true
	}

	initSetsRLS := checkStmtSetsRLS(s.Init, rlsRegex)
	thenPre := rlsActive || initSetsRLS

	if targetPos >= s.Cond.Pos() && targetPos <= s.Cond.End() {
		return thenPre, true
	}

	if s.Body != nil && targetPos >= s.Body.Pos() && targetPos <= s.Body.End() {
		return evalBlockRLS(s.Body, targetPos, thenPre, rlsRegex)
	}

	if s.Else != nil && targetPos >= s.Else.Pos() && targetPos <= s.Else.End() {
		switch el := s.Else.(type) {
		case *ast.BlockStmt:
			return evalBlockRLS(el, targetPos, thenPre, rlsRegex)
		case *ast.IfStmt:
			return evalIfRLS(el, targetPos, thenPre, rlsRegex)
		}
	}

	return rlsActive, true
}

func checkStmtSetsRLS(stmt ast.Stmt, rlsRegex *regexp.Regexp) bool {
	if stmt == nil {
		return false
	}
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		if call, ok := s.X.(*ast.CallExpr); ok && isRLSCall(call, rlsRegex) {
			return true
		}
	case *ast.AssignStmt:
		for _, rhs := range s.Rhs {
			if call, ok := rhs.(*ast.CallExpr); ok && isRLSCall(call, rlsRegex) {
				return true
			}
		}
	}
	return false
}

func evalIfJoinRLS(s *ast.IfStmt, rlsActive bool, rlsRegex *regexp.Regexp) bool {
	initSetsRLS := checkStmtSetsRLS(s.Init, rlsRegex)

	// Pattern: if _, err := db.Exec("SET LOCAL app.tenant_id = $1"); err != nil { return err }
	if initSetsRLS && s.Body != nil && isTerminatingStmt(s.Body) {
		return true
	}

	thenPre := rlsActive || initSetsRLS
	thenRLS, _ := evalBlockRLS(s.Body, token.NoPos, thenPre, rlsRegex)

	var elseRLS bool
	if s.Else != nil {
		switch el := s.Else.(type) {
		case *ast.BlockStmt:
			elseRLS, _ = evalBlockRLS(el, token.NoPos, thenPre, rlsRegex)
		case *ast.IfStmt:
			elseRLS = evalIfJoinRLS(el, thenPre, rlsRegex)
		default:
			elseRLS = rlsActive
		}
	} else {
		elseRLS = rlsActive
	}

	return thenRLS && elseRLS
}

// HasRLSSessionSetup checks if an enclosing function contains an active RLS setup at its end.
func HasRLSSessionSetup(fn *ast.FuncDecl, tenantCol string) bool {
	if fn == nil || fn.Body == nil {
		return false
	}
	return IsRLSActiveAt(fn.Body, fn.Body.End(), tenantCol)
}
