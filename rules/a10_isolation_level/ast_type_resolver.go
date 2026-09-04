// Package a10_isolation_level provides AST-level type and constructor resolution
// for environments where type checker information is absent or partial.
package a10_isolation_level

import (
	"go/ast"

	"github.com/will2469/argus/shared/callsite"
)

func findASTType(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) ast.Expr {
	if expr == nil {
		return nil
	}

	if id, ok := expr.(*ast.Ident); ok {
		if fn != nil && fn.Type != nil && fn.Type.Params != nil {
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					if name.Name == id.Name {
						return field.Type
					}
				}
			}
		}
		if fn != nil && fn.Recv != nil {
			for _, field := range fn.Recv.List {
				for _, name := range field.Names {
					if name.Name == id.Name {
						return field.Type
					}
				}
			}
		}
	}

	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if xID, ok := sel.X.(*ast.Ident); ok {
			xType := findASTType(xID, fn, file)
			structName := getASTTypeName(xType)
			if structName != "" && file != nil {
				for _, decl := range file.Decls {
					if gen, ok := decl.(*ast.GenDecl); ok {
						for _, spec := range gen.Specs {
							if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == structName {
								if st, ok := ts.Type.(*ast.StructType); ok && st.Fields != nil {
									for _, f := range st.Fields.List {
										for _, fnm := range f.Names {
											if fnm.Name == sel.Sel.Name {
												return f.Type
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
	}

	return nil
}

func isProvenDBPoolASTType(expr ast.Expr, file *ast.File) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkgID, ok := sel.X.(*ast.Ident); ok {
			if isKnownDBPackage(pkgID.Name, file) {
				switch sel.Sel.Name {
				case "DB", "Pool", "Conn":
					return true
				}
			}
		}
	}
	if id, ok := expr.(*ast.Ident); ok {
		if ts := findTypeSpec(id.Name, file); ts != nil {
			return isDBTypeSpec(ts, file)
		}
	}
	return false
}

func isDBTypeSpec(ts *ast.TypeSpec, file *ast.File) bool {
	if ts == nil || isKnownNonDBTypeName(ts.Name.Name) {
		return false
	}
	switch t := ts.Type.(type) {
	case *ast.InterfaceType:
		if t.Methods == nil {
			return false
		}
		var hasBegin, hasTxHelper bool
		for _, m := range t.Methods.List {
			if len(m.Names) == 0 {
				if isProvenDBPoolASTType(m.Type, file) {
					return true
				}
				continue
			}
			for _, name := range m.Names {
				switch name.Name {
				case "Begin", "BeginTx":
					if ft, ok := m.Type.(*ast.FuncType); ok && ft.Results != nil && len(ft.Results.List) > 0 {
						for _, res := range ft.Results.List {
							if isProvenDBTxASTType(res.Type, file) {
								hasBegin = true
								break
							}
						}
					}
				case "BeginFunc", "WithTx", "ExecuteTx":
					if ft, ok := m.Type.(*ast.FuncType); ok && ft.Params != nil {
						for _, p := range ft.Params.List {
							if cb, ok := p.Type.(*ast.FuncType); ok && cb.Params != nil && len(cb.Params.List) > 0 {
								for _, cbp := range cb.Params.List {
									if isProvenClosureTxASTType(cbp.Type, file) {
										hasTxHelper = true
										break
									}
								}
							}
						}
					}
				}
			}
		}
		return hasBegin || hasTxHelper
	case *ast.StructType:
		if t.Fields == nil {
			return false
		}
		for _, field := range t.Fields.List {
			if isProvenDBPoolASTType(field.Type, file) {
				return true
			}
		}
	}
	return false
}

func isProvenDBTxASTType(expr ast.Expr, file *ast.File) bool {
	if expr == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkgID, ok := sel.X.(*ast.Ident); ok {
			return isKnownDBPackage(pkgID.Name, file) && sel.Sel.Name == "Tx"
		}
	}
	if id, ok := expr.(*ast.Ident); ok {
		if isKnownNonDBTypeName(id.Name) {
			return false
		}
		if ts := findTypeSpec(id.Name, file); ts != nil {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok && iface.Methods != nil {
				var hasExecOrQuery, hasCommit, hasRollback bool
				for _, m := range iface.Methods.List {
					if len(m.Names) == 0 {
						if isProvenDBTxASTType(m.Type, file) {
							return true
						}
						continue
					}
					for _, name := range m.Names {
						switch name.Name {
						case "Exec", "ExecContext", "SendBatch", "Query", "QueryContext", "QueryRow", "QueryRowContext":
							hasExecOrQuery = true
						case "Commit":
							hasCommit = true
						case "Rollback":
							hasRollback = true
						}
					}
				}
				return hasCommit && hasRollback && hasExecOrQuery
			}
		}
	}
	return false
}

func isProvenClosureTxASTType(expr ast.Expr, file *ast.File) bool {
	if isProvenDBTxASTType(expr, file) {
		return true
	}
	typeName := getASTTypeName(expr)
	if isKnownNonDBTypeName(typeName) {
		return false
	}
	if id, ok := expr.(*ast.Ident); ok {
		if ts := findTypeSpec(id.Name, file); ts != nil {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok && iface.Methods != nil {
				for _, m := range iface.Methods.List {
					for _, name := range m.Names {
						switch name.Name {
						case "Exec", "ExecContext", "Query", "QueryRow", "QueryContext", "QueryRowContext", "Commit", "Rollback":
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func isDBPoolConstructorCall(call *ast.CallExpr, file *ast.File) bool {
	if call == nil || file == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return false
	}
	if pkgID, ok := sel.X.(*ast.Ident); ok {
		path := getPackageImportPath(pkgID.Name, file)
		switch path {
		case "database/sql", "github.com/jmoiron/sqlx":
			return sel.Sel.Name == "Open" || sel.Sel.Name == "OpenDB" || sel.Sel.Name == "Connect"
		case "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v4":
			return sel.Sel.Name == "Connect" || sel.Sel.Name == "ConnectConfig"
		case "github.com/jackc/pgx/v5/pgxpool", "github.com/jackc/pgx/v4/pgxpool":
			return sel.Sel.Name == "New" || sel.Sel.Name == "NewWithConfig"
		}
	}
	return false
}
