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

	enclosing := findEnclosingFunc(file, call.Pos())
	var body *ast.BlockStmt
	if enclosing != nil {
		body = enclosing.Body
	}

	results := resolveExprToStrings(dsnArg, body, file, call.Pos(), 0)
	if len(results) == 0 {
		if s, ok := callsite.ExtractQueryString(call); ok {
			results = append(results, s)
		}
	}

	return deduplicateStrings(results)
}

func resolveExprToStrings(expr ast.Expr, body *ast.BlockStmt, file *ast.File, maxPos token.Pos, depth int) []string {
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
			lefts := resolveExprToStrings(e.X, body, file, maxPos, depth+1)
			rights := resolveExprToStrings(e.Y, body, file, maxPos, depth+1)
			var combined []string
			for _, l := range lefts {
				for _, r := range rights {
					combined = append(combined, l+r)
				}
			}
			return combined
		}
	case *ast.Ident:
		var found []string
		if body != nil {
			// Find reaching assignments in enclosing function strictly prior to maxPos
			ast.Inspect(body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || assign.Pos() >= maxPos {
					return true
				}
				for i, lhs := range assign.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name == e.Name && i < len(assign.Rhs) {
						sub := resolveExprToStrings(assign.Rhs[i], body, file, assign.Pos(), depth+1)
						found = append(found, sub...)
					}
				}
				return true
			})
		}
		if len(found) > 0 {
			return found
		}

		// Fallback to package-level declarations
		if file != nil {
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
					for i, name := range valSpec.Names {
						if name.Name == e.Name && i < len(valSpec.Values) {
							sub := resolveExprToStrings(valSpec.Values[i], body, file, maxPos, depth+1)
							found = append(found, sub...)
						}
					}
				}
			}
		}
		return found
	case *ast.ParenExpr:
		return resolveExprToStrings(e.X, body, file, maxPos, depth+1)
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
