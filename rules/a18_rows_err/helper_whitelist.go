// Package a18_rows_err implements the ARGUS-A18 static analysis rule.
package a18_rows_err

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

// IsSafeCollectionHelper determines if a CallExpr is an official auto-closing helper
// (such as pgx.CollectRows, pgx.CollectOneRow, pgx.ForEachRow) that handles rows.Err() internally.
func IsSafeCollectionHelper(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	methodName := callsite.GetCallMethodName(call.Fun)
	switch methodName {
	case "CollectRows", "CollectOneRow", "ForEachRow":
		return true
	}
	return false
}

// HasDatabaseImports checks if an AST file imports known database packages.
func HasDatabaseImports(file *ast.File) bool {
	if file == nil {
		return true
	}
	for _, imp := range file.Imports {
		if imp.Path == nil {
			continue
		}
		p := strings.Trim(imp.Path.Value, `"`)
		if strings.Contains(p, "pgx") || strings.Contains(p, "database/sql") || strings.Contains(p, "argus") || strings.Contains(p, "pgconn") {
			return true
		}
	}
	return false
}

// IsDatabaseRowsReceiver determines if an expression receiver is likely a database pgx.Rows / sql.Rows.
func IsDatabaseRowsReceiver(pass *analysis.Pass, expr ast.Expr) bool {
	if expr == nil {
		return false
	}

	if pass != nil && pass.TypesInfo != nil {
		if tv, ok := pass.TypesInfo.Types[expr]; ok && tv.Type != nil {
			typStr := strings.ToLower(tv.Type.String())
			if strings.Contains(typStr, "testing.pb") || strings.Contains(typStr, "testing.b") || strings.Contains(typStr, "bufio.scanner") || strings.Contains(typStr, "excelize") {
				return false
			}
			if strings.Contains(typStr, "rows") || strings.Contains(typStr, "row") || strings.Contains(typStr, "pgx") || strings.Contains(typStr, "sql") {
				return true
			}
		}
	}

	// Heuristic check on receiver identifier name
	if ident, ok := expr.(*ast.Ident); ok {
		lower := strings.ToLower(ident.Name)
		if lower == "pb" || lower == "scanner" || lower == "benchmark" || lower == "chan" || lower == "ch" {
			return false
		}
		if strings.Contains(lower, "row") || strings.Contains(lower, "r") || strings.Contains(lower, "cursor") || strings.Contains(lower, "iter") {
			return true
		}
	}

	return true
}
