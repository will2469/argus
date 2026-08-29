package a17_nplusone

import (
	"go/ast"
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

	// If types info is available, verify receiver type
	if pass != nil && pass.TypesInfo != nil {
		if selType, ok := pass.TypesInfo.Selections[sel]; ok {
			recvType := selType.Recv().String()
			return isDatabaseReceiverType(recvType)
		}
		if tv, ok := pass.TypesInfo.Types[sel.X]; ok {
			recvType := tv.Type.String()
			return isDatabaseReceiverType(recvType)
		}
	}

	// Heuristic fallback if types info is unavailable:
	// If receiver variable name is clearly non-DB, or args don't look like db query args
	recvName := ""
	if id, ok := sel.X.(*ast.Ident); ok {
		recvName = strings.ToLower(id.Name)
	}
	if recvName == "cmd" || recvName == "command" || recvName == "os" || recvName == "exec" {
		return false
	}

	return true
}

func isDatabaseReceiverType(recvType string) bool {
	lower := strings.ToLower(recvType)
	if strings.Contains(lower, "pgx") || strings.Contains(lower, "database/sql") {
		return true
	}
	for _, part := range strings.FieldsFunc(lower, func(r rune) bool {
		return r == '.' || r == '*' || r == '/' || r == '_'
	}) {
		switch part {
		case "db", "pool", "tx", "conn", "querier", "database", "store", "repo", "repository":
			return true
		}
	}
	return false
}
