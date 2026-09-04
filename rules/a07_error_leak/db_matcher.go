// Package a07_error_leak provides semantic recognition of database callsites
// based strictly on compiler types, interface method sets, and AST declarations,
// with zero reliance on fragile receiver variable naming heuristics.
package a07_error_leak

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

// isDatabaseCall determines whether a call expression is an operation on a genuine database connection,
// pool, transaction, or driver package.
func isDatabaseCall(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, call *ast.CallExpr) bool {
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

	// 1. Semantic Type Verification via pass.TypesInfo
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

		// Package-level calls from database packages: sql.Open, pgx.Connect, etc.
		if id, ok := sel.X.(*ast.Ident); ok {
			if pkgName, ok := pass.TypesInfo.Uses[id].(*types.PkgName); ok {
				return isKnownDBPackagePath(pkgName.Imported().Path())
			}
		}
	}

	// 2. Standalone Mode (pass == nil or TypesInfo unavailable)
	// Fail-closed: only check known database package identifiers
	if id, ok := sel.X.(*ast.Ident); ok {
		switch id.Name {
		case "sql", "pgx", "pgxpool", "sqlx", "pq":
			return true
		}

		// Check AST declaration for proven DB type
		decl := findASTDeclForIdent(file, fn, id)
		if decl != nil {
			return isKnownDBASTDecl(decl)
		}
	}

	// Fail-closed: if we cannot prove it's a DB call, don't check
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

func isKnownDBASTDecl(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.Field:
		return isKnownDBASTType(n.Type)
	case *ast.ValueSpec:
		return isKnownDBASTType(n.Type)
	case *ast.AssignStmt:
		for _, rhs := range n.Rhs {
			if rhsCall, ok := rhs.(*ast.CallExpr); ok && isDBConstructorCall(rhsCall) {
				return true
			}
		}
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

func findASTDeclForIdent(file *ast.File, fn *ast.FuncDecl, id *ast.Ident) ast.Node {
	if id == nil {
		return nil
	}
	if id.Obj != nil && id.Obj.Decl != nil {
		return id.Obj.Decl.(ast.Node)
	}

	if fn != nil {
		if fn.Type != nil && fn.Type.Params != nil {
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					if name.Name == id.Name {
						return field
					}
				}
			}
		}

		if fn.Body != nil {
			blocks := getEnclosingBlocks(fn.Body, id.Pos())
			for i := len(blocks) - 1; i >= 0; i-- {
				b := blocks[i]
				for _, stmt := range b.List {
					if stmt.Pos() >= id.Pos() {
						continue
					}
					switch s := stmt.(type) {
					case *ast.AssignStmt:
						for _, lhs := range s.Lhs {
							if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name {
								return s
							}
						}
					case *ast.DeclStmt:
						if gen, ok := s.Decl.(*ast.GenDecl); ok {
							for _, spec := range gen.Specs {
								if valSpec, ok := spec.(*ast.ValueSpec); ok {
									for _, name := range valSpec.Names {
										if name.Name == id.Name {
											return valSpec
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if file != nil {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range gen.Specs {
					if valSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valSpec.Names {
							if name.Name == id.Name {
								return valSpec
							}
						}
					}
				}
			}
		}
	}

	return nil
}
