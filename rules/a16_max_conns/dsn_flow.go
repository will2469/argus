// Package a16_max_conns tracks DSN expressions and transitive aliases for pgxpool calls.
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

	results := resolveExprToStringsWithFlow(file, dsnArg, call.Pos(), 0)
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
	curr := inSet
	for _, stmt := range block.List {
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
	if targetPos != token.NoPos && targetPos < stmt.Pos() {
		return inSet, true
	}

	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if targetPos != token.NoPos && s.Pos() <= targetPos && targetPos <= s.End() {
			return inSet, true
		}
		for i, lhs := range s.Lhs {
			if id, ok := lhs.(*ast.Ident); ok && id.Name == varName && i < len(s.Rhs) {
				newVals := resolveExprToStringsWithFlow(file, s.Rhs[i], stmt.Pos(), depth+1)
				if len(newVals) > 0 {
					inSet = newVals
				}
			}
		}
		return inSet, false

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

		thenSet, _ := evalDSNBlockFlow(file, s.Body, targetPos, varName, inSet, depth)
		thenTerm := isTerminating(s.Body)

		var elseSet []string
		elseTerm := false
		if s.Else != nil {
			elseSet, _ = evalDSNStmtFlow(file, s.Else, targetPos, varName, inSet, depth)
			elseTerm = isTerminating(s.Else)
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
	}

	return inSet, false
}

func findPackageLevelString(file *ast.File, name string) []string {
	if file == nil {
		return nil
	}
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
					return resolveExprToStringsWithFlow(file, valSpec.Values[i], token.NoPos, 0)
				}
			}
		}
	}
	return nil
}

func deduplicateStrings(items []string) []string {
	if len(items) <= 1 {
		return items
	}
	seen := make(map[string]bool)
	var out []string
	for _, it := range items {
		if !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
	}
	return out
}
