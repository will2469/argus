// Package callsite provides AST recognition utilities for database operations,
// supporting modern Go 1.22-1.26+ idioms including generic call wrappers and string concatenation.
package callsite

import (
	"go/ast"
	"go/token"
	"go/types"

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

// Known standard and popular Go SQL driver / pool package paths.
var knownDBPackagePaths = map[string]bool{
	"database/sql":                    true,
	"github.com/jackc/pgx/v5":         true,
	"github.com/jackc/pgx/v5/pgxpool": true,
	"github.com/jackc/pgx/v5/pgconn":  true,
	"github.com/jackc/pgx/v4":         true,
	"github.com/jackc/pgx/v4/pgxpool": true,
	"github.com/jackc/pgx/v4/pgconn":  true,
	"github.com/jmoiron/sqlx":         true,
	"github.com/lib/pq":               true,
}

// IsPgxOrSQLType verifies if a Go type represents a legitimate database connection, pool, or transaction.
// It inspects package path, exact type identity, and interface methods rather than naive substring matching.
func IsPgxOrSQLType(t types.Type) bool {
	if t == nil {
		return false
	}

	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}

	// 1. Exact package path and type name match for known database drivers
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil {
			pkg := obj.Pkg()
			if pkg != nil && knownDBPackagePaths[pkg.Path()] {
				switch obj.Name() {
				case "DB", "Tx", "Conn", "Pool", "Batch", "Stmt", "Rows", "Row":
					return true
				}
			}
		}
	}

	// 2. Interface method set check for database querier contracts (e.g. DBTX, Querier)
	if hasDBMethodSet(t) {
		return true
	}

	return false
}

func hasDBMethodSet(t types.Type) bool {
	var hasQuery, hasExec bool

	checkFunc := func(fn *types.Func) {
		name := fn.Name()
		switch name {
		case "Query", "QueryRow":
			hasQuery = true
		case "Exec", "ExecContext", "Begin", "BeginTx", "SendBatch":
			hasExec = true
		}
	}

	if named, ok := t.(*types.Named); ok {
		for i := 0; i < named.NumMethods(); i++ {
			checkFunc(named.Method(i))
		}
	}

	if iface, ok := t.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			checkFunc(iface.Method(i))
		}
	}

	return hasQuery && hasExec
}

