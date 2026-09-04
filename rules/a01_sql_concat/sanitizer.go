package a01_sql_concat

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// IsSanitized returns true if the expression is protected by an approved identifier sanitizer.
// It enforces strict identity checking:
// 1. pgx.Identifier.Sanitize() (verified receiver type or AST structure)
// 2. Standalone functions named SanitizeIdentifier, QuoteIdentifier, QuoteIdent
// 3. Methods on approved database driver/utility receivers (pgx, pq, sql, db)
// Arbitrary structs with a .Sanitize(...) method (e.g. evil.Sanitize) are strictly rejected.
func IsSanitized(e ast.Expr, pass *analysis.Pass) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}

	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		method := fn.Sel.Name

		// 1. pgx.Identifier.Sanitize()
		if method == "Sanitize" {
			if pass != nil && pass.TypesInfo != nil {
				if tv, ok := pass.TypesInfo.Types[fn.X]; ok && tv.Type != nil {
					typeStr := tv.Type.String()
					if strings.Contains(typeStr, "pgx") && strings.HasSuffix(typeStr, "Identifier") {
						return true
					}
				}
			}
			// AST fallback: verify receiver expression is an Identifier
			if isPgxIdentifierExpr(fn.X) {
				return true
			}
			return false
		}

		// 2. Qualified identifier sanitizers like pq.QuoteIdentifier, pgx.QuoteIdentifier
		if method == "SanitizeIdentifier" || method == "QuoteIdentifier" || method == "QuoteIdent" {
			return isTrustedSanitizerReceiver(fn.X, pass)
		}

	case *ast.Ident:
		// Standalone package-level sanitizer functions
		switch fn.Name {
		case "SanitizeIdentifier", "QuoteIdentifier", "QuoteIdent":
			return true
		}
	}

	return false
}

func isTrustedSanitizerReceiver(recv ast.Expr, pass *analysis.Pass) bool {
	if id, ok := recv.(*ast.Ident); ok {
		name := strings.ToLower(id.Name)
		switch name {
		case "pgx", "pq", "sql", "driver", "sanitizer":
			return true
		}
	}
	if sel, ok := recv.(*ast.SelectorExpr); ok {
		return isTrustedSanitizerReceiver(sel.Sel, pass)
	}
	if pass != nil && pass.TypesInfo != nil {
		if tv, ok := pass.TypesInfo.Types[recv]; ok && tv.Type != nil {
			typeStr := strings.ToLower(tv.Type.String())
			if strings.Contains(typeStr, "pgx") || strings.Contains(typeStr, "pq") || strings.Contains(typeStr, "sql") {
				return true
			}
		}
	}
	return false
}

func isPgxIdentifierExpr(e ast.Expr) bool {
	if comp, ok := e.(*ast.CompositeLit); ok {
		if sel, ok := comp.Type.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "pgx" && sel.Sel.Name == "Identifier" {
				return true
			}
		}
		if id, ok := comp.Type.(*ast.Ident); ok {
			return id.Name == "Identifier"
		}
	}
	if sel, ok := e.(*ast.SelectorExpr); ok {
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "pgx" && sel.Sel.Name == "Identifier" {
			return true
		}
	}
	return false
}
