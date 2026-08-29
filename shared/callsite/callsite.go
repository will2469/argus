// Package callsite provides AST recognition utilities for database operations,
// supporting modern Go 1.22-1.26+ idioms including generic call wrappers and string concatenation.
package callsite

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Known database query method names.
var queryMethods = map[string]bool{
	"Query":       true,
	"QueryRow":    true,
	"Exec":        true,
	"ExecContext": true,
	"SendBatch":   true,
	"CopyFrom":    true,
	"Begin":       true,
	"BeginTx":     true,
	"BeginFunc":   true,
	"Prepare":     true,
}

// IsDBQueryMethod checks if a method identifier belongs to common database interfaces.
func IsDBQueryMethod(name string) bool {
	return queryMethods[name]
}

// GetCallSelector extracts the *ast.SelectorExpr from a call target expression,
// seamlessly unwrapping modern Go generic type parameters (e.g. repo.Query[User](...) or db.Exec[K, V](...)).
func GetCallSelector(fun ast.Expr) *ast.SelectorExpr {
	switch e := fun.(type) {
	case *ast.SelectorExpr:
		return e
	case *ast.IndexExpr: // generic call with 1 type argument: db.Query[User](...)
		if sel, ok := e.X.(*ast.SelectorExpr); ok {
			return sel
		}
	case *ast.IndexListExpr: // generic call with multiple type arguments: db.Query[K, V](...)
		if sel, ok := e.X.(*ast.SelectorExpr); ok {
			return sel
		}
	}
	return nil
}

// GetCallMethodName returns the method name of a call expression, unwrapping generic type instantiations.
func GetCallMethodName(fun ast.Expr) string {
	if sel := GetCallSelector(fun); sel != nil {
		return sel.Sel.Name
	}
	if id, ok := fun.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// ExtractQueryString extracts a compile-time SQL string literal from arguments of a DB call.
// Resolves inline literals, concatenated strings ("a" + "b"), local variables, and constants.
func ExtractQueryString(call *ast.CallExpr) (string, bool) {
	if call == nil || len(call.Args) == 0 {
		return "", false
	}

	for _, arg := range call.Args {
		if s, ok := extractString(arg); ok {
			return s, true
		}
	}
	return "", false
}

func extractString(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			val := e.Value
			if len(val) >= 2 && ((val[0] == '`' && val[len(val)-1] == '`') ||
				(val[0] == '"' && val[len(val)-1] == '"')) {
				return val[1 : len(val)-1], true
			}
		}
	case *ast.BinaryExpr:
		// Support compile-time string concatenation: "SELECT id " + "FROM users"
		if e.Op == token.ADD {
			s1, ok1 := extractString(e.X)
			s2, ok2 := extractString(e.Y)
			if ok1 && ok2 {
				return s1 + s2, true
			}
		}
	case *ast.Ident:
		if e.Obj != nil {
			switch decl := e.Obj.Decl.(type) {
			case *ast.AssignStmt:
				for i, lhs := range decl.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name == e.Name && i < len(decl.Rhs) {
						return extractString(decl.Rhs[i])
					}
				}
			case *ast.ValueSpec:
				for i, name := range decl.Names {
					if name.Name == e.Name && i < len(decl.Values) {
						return extractString(decl.Values[i])
					}
				}
			}
		}
	}
	return "", false
}

// IsInsideLoop checks if an AST node path is enclosed within a ForStmt or RangeStmt.
func IsInsideLoop(path []ast.Node) bool {
	for _, n := range path {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		}
	}
	return false
}

// GetEnclosingFunc returns the innermost FuncDecl or FuncLit enclosing the node.
func GetEnclosingFunc(path []ast.Node) ast.Node {
	for i := len(path) - 1; i >= 0; i-- {
		switch fn := path[i].(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return fn
		}
	}
	return nil
}

// Analyzer provides shared callsite analysis helper.
var Analyzer = &analysis.Analyzer{
	Name: "argus_callsite",
	Doc:  "Identifies database client callsites (*pgxpool.Pool, pgx.Tx, *sql.DB)",
	Run:  runCallsiteAnalyzer,
}

func runCallsiteAnalyzer(pass *analysis.Pass) (interface{}, error) {
	return nil, nil
}

// IsPgxOrSQLType heuristically checks if a type or identifier matches database pools/conns.
func IsPgxOrSQLType(t types.Type) bool {
	if t == nil {
		return false
	}
	s := t.String()
	if len(s) == 0 {
		return false
	}
	return strings.Contains(s, "pgx") ||
		strings.Contains(s, "database/sql") ||
		strings.Contains(s, "pgxpool") ||
		strings.Contains(s, "Pool") ||
		strings.Contains(s, "Tx") ||
		strings.Contains(s, "DB") ||
		strings.Contains(s, "Conn") ||
		strings.Contains(s, "Querier")
}
