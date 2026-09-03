package a17_nplusone

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

// IsDBQueryCall determines if a CallExpr executes a database query.
// Uses pass.TypesInfo when available to rule out non-database receivers.
func IsDBQueryCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if call == nil {
		return false
	}

	methodName := callsite.GetCallMethodName(call.Fun)
	if !callsite.IsDBQueryMethod(methodName) {
		return false
	}

	// Must have at least one argument (typically ctx or SQL query)
	if len(call.Args) == 0 {
		return false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// 1. If types info is available, verify receiver type
	if pass != nil && pass.TypesInfo != nil {
		var recvType types.Type
		if selType, ok := pass.TypesInfo.Selections[sel]; ok {
			recvType = selType.Recv()
		} else if tv, ok := pass.TypesInfo.Types[sel.X]; ok {
			recvType = tv.Type
		}

		if recvType != nil {
			if callsite.IsPgxOrSQLType(recvType) {
				return true
			}
			typeName := extractNamedTypeName(recvType)
			if typeName != "" {
				lowerType := strings.ToLower(typeName)
				if isNonDatabaseReceiverName(lowerType) {
					return false
				}
				if isDatabaseReceiverName(lowerType) {
					return true
				}
			}
		}
	}

	// 2. Heuristic fallback: check receiver identifier or SQL query argument
	recvName := ""
	switch x := sel.X.(type) {
	case *ast.Ident:
		recvName = strings.ToLower(x.Name)
	case *ast.SelectorExpr:
		recvName = strings.ToLower(x.Sel.Name)
	}

	// Explicit reject for known non-database receivers
	if isNonDatabaseReceiverName(recvName) {
		return false
	}

	// Positive match: receiver name indicates database operations
	if isDatabaseReceiverName(recvName) {
		return true
	}

	// Positive match: one of the call arguments looks like a SQL query string
	if hasSQLQueryArgument(call) {
		return true
	}

	return false
}

func extractNamedTypeName(t types.Type) string {
	if t == nil {
		return ""
	}
	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}

func isNonDatabaseReceiverName(name string) bool {
	switch name {
	case "cmd", "command", "os", "exec",
		"search", "searchengine", "client", "httpclient", "http",
		"metrics", "solr", "es", "engine", "req", "request", "url",
		"log", "logger", "trace", "tracer", "cache", "redis", "memcache",
		"memorycache":
		return true
	}
	return false
}

func isDatabaseReceiverName(name string) bool {
	switch name {
	case "db", "pool", "tx", "conn", "queries", "querier", "database", "store", "repo", "repository", "r":
		return true
	}
	for _, suffix := range []string{"db", "pool", "tx", "conn", "repo", "store"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func hasSQLQueryArgument(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	for _, arg := range call.Args {
		if s, ok := extractSimpleString(arg); ok {
			upper := strings.ToUpper(strings.TrimSpace(s))
			if strings.HasPrefix(upper, "SELECT") ||
				strings.HasPrefix(upper, "INSERT") ||
				strings.HasPrefix(upper, "UPDATE") ||
				strings.HasPrefix(upper, "DELETE") ||
				strings.HasPrefix(upper, "WITH") {
				return true
			}
		}
	}
	return false
}

func extractSimpleString(expr ast.Expr) (string, bool) {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING && len(lit.Value) >= 2 {
		return lit.Value[1 : len(lit.Value)-1], true
	}
	return "", false
}
