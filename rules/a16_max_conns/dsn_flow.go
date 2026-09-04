// Package a16_max_conns tracks DSN expressions and reaching definitions for pgxpool calls.
package a16_max_conns

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/will2469/argus/shared/callsite"
)

// extractAllDSNStrings resolves all compile-time DSN string values reaching a pgxpool call,
// supporting direct literals, concatenations, transitive aliases, and reaching reassignments.
func extractAllDSNStrings(call *ast.CallExpr, file *ast.File) []string {
	if call == nil || len(call.Args) == 0 {
		return nil
	}

	dsnArg, _ := findCallArg(call, nil)
	if dsnArg == nil {
		return nil
	}

	results := resolveExprToStringsWithFlow(file, dsnArg, dsnArg.Pos(), 0)
	if len(results) == 0 {
		if s, ok := callsite.ExtractQueryString(call); ok {
			results = append(results, s)
		}
	}

	return deduplicateStrings(results)
}

func resolveExprToStringsWithFlow(file *ast.File, expr ast.Expr, pos token.Pos, depth int) []string {
	if expr == nil || depth > 10 {
		return nil
	}

	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return []string{strings.Trim(e.Value, "`\"")}
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			lefts := resolveExprToStringsWithFlow(file, e.X, pos, depth+1)
			rights := resolveExprToStringsWithFlow(file, e.Y, pos, depth+1)
			var combined []string
			for _, l := range lefts {
				for _, r := range rights {
					combined = append(combined, l+r)
				}
			}
			return combined
		}
	case *ast.Ident:
		enclosing := findEnclosingFunc(file, pos)
		if enclosing != nil && enclosing.Body != nil {
			reached, _ := evalDSNBlockFlow(file, enclosing.Body, pos, e.Name, nil, depth+1)
			if len(reached) > 0 {
				return reached
			}
		}
		return findPackageLevelString(file, e.Name)
	case *ast.ParenExpr:
		return resolveExprToStringsWithFlow(file, e.X, pos, depth+1)
	}

	return nil
}

func evalDSNBlockFlow(file *ast.File, block *ast.BlockStmt, targetPos token.Pos, varName string, inSet []string, depth int) ([]string, bool) {
	if block == nil || depth > 10 {
		return inSet, false
	}
	return evalDSNStmtList(file, block.List, targetPos, varName, inSet, depth)
}

func evalDSNStmtList(file *ast.File, list []ast.Stmt, targetPos token.Pos, varName string, inSet []string, depth int) ([]string, bool) {
	curr := inSet
	for _, stmt := range list {
		var reached bool
		curr, reached = evalDSNStmtFlow(file, stmt, targetPos, varName, curr, depth)
		if reached {
			return curr, true
		}
	}
	return curr, false
}

func evalDSNStmtFlow(file *ast.File, stmt ast.Stmt, targetPos token.Pos, varName string, inSet []string, depth int) ([]string, bool) {
	if stmt == nil {
		return inSet, false
	}
	if targetPos != token.NoPos && targetPos <= stmt.Pos() {
		return inSet, true
	}

	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if targetPos != token.NoPos && exprListContainsPos(s.Rhs, targetPos) {
			return inSet, true
		}
		for i, lhs := range s.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == varName && i < len(s.Rhs) {
				inSet = resolveExprToStringsWithFlow(file, s.Rhs[i], s.Pos(), depth+1)
			}
		}
		if targetPos != token.NoPos && targetPos <= s.End() {
			return inSet, true
		}
		return inSet, false

	case *ast.DeclStmt:
		if gen, ok := s.Decl.(*ast.GenDecl); ok {
			for _, spec := range gen.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					if targetPos != token.NoPos && exprListContainsPos(vs.Values, targetPos) {
						return inSet, true
					}
					for i, name := range vs.Names {
						if name.Name == varName && i < len(vs.Values) {
							inSet = resolveExprToStringsWithFlow(file, vs.Values[i], s.Pos(), depth+1)
						}
					}
				}
			}
		}
		if targetPos != token.NoPos && targetPos <= s.End() {
			return inSet, true
		}
		return inSet, false

	case *ast.ExprStmt:
		if targetPos != token.NoPos && targetPos <= s.End() {
			return inSet, true
		}
		return inSet, false

	case *ast.BlockStmt:
		if targetPos != token.NoPos && s.Pos() <= targetPos && targetPos <= s.End() {
			return evalDSNBlockFlow(file, s, targetPos, varName, inSet, depth)
		}
		if blockShadowsVar(s, varName) {
			return inSet, false
		}
		return evalDSNBlockFlow(file, s, targetPos, varName, inSet, depth)

	case *ast.IfStmt:
		if s.Init != nil {
			var reached bool
			inSet, reached = evalDSNStmtFlow(file, s.Init, targetPos, varName, inSet, depth)
			if reached {
				return inSet, true
			}
		}
		if targetPos != token.NoPos && s.Body != nil && s.Body.Pos() <= targetPos && targetPos <= s.Body.End() {
			return evalDSNBlockFlow(file, s.Body, targetPos, varName, inSet, depth)
		}
		if targetPos != token.NoPos && s.Else != nil && s.Else.Pos() <= targetPos && targetPos <= s.Else.End() {
			return evalDSNStmtFlow(file, s.Else, targetPos, varName, inSet, depth)
		}

		var thenSet []string
		thenTerm := false
		if s.Body != nil {
			if blockShadowsVar(s.Body, varName) {
				thenSet = inSet
			} else {
				thenSet, _ = evalDSNBlockFlow(file, s.Body, targetPos, varName, inSet, depth)
				thenTerm = isTerminating(s.Body)
			}
		} else {
			thenSet = inSet
		}

		var elseSet []string
		elseTerm := false
		if s.Else != nil {
			if stmtShadowsVar(s.Else, varName) {
				elseSet = inSet
			} else {
				elseSet, _ = evalDSNStmtFlow(file, s.Else, targetPos, varName, inSet, depth)
				elseTerm = isTerminating(s.Else)
			}
		} else {
			elseSet = inSet
		}

		if thenTerm && !elseTerm {
			return elseSet, false
		}
		if elseTerm && !thenTerm {
			return thenSet, false
		}
		return deduplicateStrings(append(append([]string{}, thenSet...), elseSet...)), false

	case *ast.SwitchStmt:
		return evalDSNSwitchFlow(file, s, targetPos, varName, inSet, depth)
	}

	return inSet, false
}
