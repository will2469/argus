// Package a07_error_leak provides symbol matching and call classification for database and non-database operations.
package a07_error_leak

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

func isDatabaseCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return false
	}

	methodName := sel.Sel.Name
	if callsite.IsDBQueryMethod(methodName) {
		return true
	}

	// Transaction and scanning methods
	switch methodName {
	case "Commit", "Rollback", "Scan", "Err", "Ping", "Close":
		return true
	}

	// Type-based receiver verification
	if pass != nil && pass.TypesInfo != nil {
		if recvType := pass.TypesInfo.TypeOf(sel.X); recvType != nil {
			if callsite.IsPgxOrSQLType(recvType) {
				return true
			}
		}
	}

	// Receiver naming heuristic for repos and DAOs
	if id, ok := sel.X.(*ast.Ident); ok {
		lower := strings.ToLower(id.Name)
		if strings.Contains(lower, "repo") || strings.Contains(lower, "store") ||
			strings.Contains(lower, "dao") || strings.Contains(lower, "queries") ||
			strings.Contains(lower, "db") || strings.Contains(lower, "pool") || strings.Contains(lower, "tx") {
			return true
		}
	}

	return false
}

func isNonDatabaseCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel := callsite.GetCallSelector(call.Fun)
	var methodName string
	if sel != nil {
		methodName = sel.Sel.Name
	} else if id, ok := call.Fun.(*ast.Ident); ok {
		methodName = id.Name
	}

	lowerMethod := strings.ToLower(methodName)
	nonDBMethodKeywords := []string{
		"validate", "validation", "check", "sanitize", "parse",
		"decode", "unmarshal", "verify", "auth", "bind", "normalize",
	}
	for _, kw := range nonDBMethodKeywords {
		if strings.Contains(lowerMethod, kw) {
			return true
		}
	}

	// Check package path via types or identifier
	if sel != nil {
		if pass != nil && pass.TypesInfo != nil {
			if id, ok := sel.X.(*ast.Ident); ok {
				if pkgName, ok := pass.TypesInfo.Uses[id].(*types.PkgName); ok {
					switch pkgName.Imported().Path() {
					case "encoding/json", "encoding/xml", "encoding/csv", "encoding/base64",
						"encoding/hex", "strconv", "os", "io", "net/url", "time", "errors", "fmt":
						return true
					}
				}
			}
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			switch id.Name {
			case "json", "xml", "csv", "base64", "hex", "strconv", "os", "io", "url", "time", "errors", "fmt":
				return true
			}
		}
	}

	return false
}

func isNonDBErrorName(name string) bool {
	nonDBPatterns := []string{
		"validation", "valerr", "clienterr", "inputerr", "parseerr",
		"autherr", "formerr", "reqerr", "userfacing", "usererr", "badreq", "schemaerr",
	}
	for _, p := range nonDBPatterns {
		if strings.Contains(name, p) {
			return true
		}
	}
	return false
}

func isDBErrorName(name string) bool {
	dbPatterns := []string{
		"dberr", "pgerr", "sqlerr", "txerr", "queryerr", "repoerr", "storeerr",
	}
	for _, p := range dbPatterns {
		if strings.Contains(name, p) {
			return true
		}
	}
	return false
}
