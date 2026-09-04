// Package a04_orderby traces data flow of variables interpolated into SQL ORDER BY clauses,
// enforcing that dynamic sort identifiers originate from closed-set allowlist maps or switch-case branches.
package a04_orderby

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// IsSafeOrderBy checks whether a variable used in an ORDER BY clause is safely mapped
// from a verified allowlist map lookup or an exhaustive switch-case statement.
func IsSafeOrderBy(ident *ast.Ident, body *ast.BlockStmt, file *ast.File, pass *analysis.Pass, allowedCols []string) bool {
	if ident == nil || body == nil {
		return false
	}

	var hasSafeAssignment bool
	var hasUnsafeAssignment bool

	ast.Inspect(body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range stmt.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name != ident.Name || i >= len(stmt.Rhs) {
					continue
				}
				rhs := stmt.Rhs[i]

				// 1. Allowlist map lookup: val, ok := sortMap[userSort]
				if idx, isIndex := rhs.(*ast.IndexExpr); isIndex {
					if isAllowlistMapLookup(idx, body, file, pass, allowedCols) {
						hasSafeAssignment = true
					} else {
						hasUnsafeAssignment = true
					}
					continue
				}

				// 2. Static string literal assignment: col = "created_at"
				if lit, isLit := rhs.(*ast.BasicLit); isLit && lit.Kind == token.STRING {
					if len(allowedCols) > 0 {
						if isColAllowed(unquoteString(lit.Value), allowedCols) {
							hasSafeAssignment = true
						} else {
							hasUnsafeAssignment = true
						}
					} else {
						hasSafeAssignment = true
					}
					continue
				}

				// Any other assignment is considered untrusted
				hasUnsafeAssignment = true
			}

		case *ast.SwitchStmt:
			isSafe, hasUnsafe := isExhaustiveSafeSwitch(stmt, ident.Name, body, file, pass, allowedCols)
			if isSafe {
				hasSafeAssignment = true
			}
			if hasUnsafe {
				hasUnsafeAssignment = true
			}
		}

		return true
	})

	return hasSafeAssignment && !hasUnsafeAssignment
}
