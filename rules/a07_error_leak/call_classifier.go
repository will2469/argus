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
	if call == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return false
	}

	methodName := sel.Sel.Name
	if !isCandidateDBMethod(methodName) {
		return false
	}

	// 1. Semantic Type-Based Verification via pass.TypesInfo
	if pass != nil && pass.TypesInfo != nil {
		var recvType types.Type
		if selType, ok := pass.TypesInfo.Selections[sel]; ok && selType.Recv() != nil {
			recvType = selType.Recv()
		} else if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.Type != nil {
			recvType = tv.Type
		} else if id, ok := sel.X.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				recvType = obj.Type()
			}
		}

		if recvType != nil && recvType != types.Typ[types.Invalid] {
			return callsite.IsPgxOrSQLType(recvType)
		}

		// Package-level calls from database packages, e.g. sql.Open, pgx.Connect
		if id, ok := sel.X.(*ast.Ident); ok {
			if pkgName, ok := pass.TypesInfo.Uses[id].(*types.PkgName); ok {
				return isKnownDBPackagePath(pkgName.Imported().Path())
			}
		}
	}

	// 2. AST-level Symbol / Type Verification (when pass == nil or TypesInfo is unavailable)
	if id, ok := sel.X.(*ast.Ident); ok {
		switch id.Name {
		case "sql", "pgx", "pgxpool", "sqlx", "pq":
			return true
		}

		if id.Obj != nil {
			if field, ok := id.Obj.Decl.(*ast.Field); ok && isKnownDBASTType(field.Type) {
				return true
			}
			if vs, ok := id.Obj.Decl.(*ast.ValueSpec); ok && isKnownDBASTType(vs.Type) {
				return true
			}
			if as, ok := id.Obj.Decl.(*ast.AssignStmt); ok {
				for i, lhs := range as.Lhs {
					if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name {
						var rhs ast.Expr
						if i < len(as.Rhs) {
							rhs = as.Rhs[i]
						} else if len(as.Rhs) == 1 {
							rhs = as.Rhs[0]
						}
						if rhsCall, ok := rhs.(*ast.CallExpr); ok && isDBConstructorCall(rhsCall) {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

func isCandidateDBMethod(methodName string) bool {
	if callsite.IsDBQueryMethod(methodName) {
		return true
	}
	switch methodName {
	case "Commit", "Rollback", "Scan", "Err", "Ping", "PingContext",
		"Close", "SendBatch", "Prepare", "PrepareContext", "CopyFrom",
		"Begin", "BeginTx", "BeginTxFunc":
		return true
	}
	return false
}

func isKnownDBPackagePath(path string) bool {
	switch path {
	case "database/sql", "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v5/pgxpool",
		"github.com/jackc/pgx/v5/pgconn", "github.com/jackc/pgx/v4", "github.com/jackc/pgx/v4/pgxpool",
		"github.com/jmoiron/sqlx", "github.com/lib/pq":
		return true
	}
	return false
}

func isKnownDBASTType(expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkgID, ok := sel.X.(*ast.Ident); ok {
			switch pkgID.Name {
			case "sql", "pgx", "pgxpool", "sqlx", "pq":
				switch sel.Sel.Name {
				case "DB", "Tx", "Conn", "Pool", "Row", "Rows", "Stmt", "Batch":
					return true
				}
			}
		}
	}
	if id, ok := expr.(*ast.Ident); ok {
		switch id.Name {
		case "DB", "Tx", "Pool", "Querier", "DBTX":
			return true
		}
	}
	return false
}

func isDBConstructorCall(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return false
	}
	if pkgID, ok := sel.X.(*ast.Ident); ok {
		switch pkgID.Name {
		case "sql":
			return sel.Sel.Name == "Open" || sel.Sel.Name == "OpenDB"
		case "pgx":
			return sel.Sel.Name == "Connect" || sel.Sel.Name == "ConnectConfig"
		case "pgxpool":
			return sel.Sel.Name == "New" || sel.Sel.Name == "NewWithConfig"
		case "sqlx":
			return sel.Sel.Name == "Open" || sel.Sel.Name == "Connect"
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
