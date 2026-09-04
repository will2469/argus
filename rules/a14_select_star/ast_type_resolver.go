// Package a14_select_star provides AST-level type and constructor resolution
// for environments where type checker information is absent or partial.
package a14_select_star

import (
	"go/ast"
	"go/token"

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
		if fn != nil && fn.Body != nil {
			var declType ast.Expr
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if declType != nil {
					return false
				}
				switch s := n.(type) {
				case *ast.ValueSpec:
					for _, nm := range s.Names {
						if nm.Name == id.Name && s.Type != nil {
							declType = s.Type
							return false
						}
					}
				case *ast.AssignStmt:
					for i, lhs := range s.Lhs {
						if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name && i < len(s.Rhs) {
							if rhsComp, ok := s.Rhs[i].(*ast.CompositeLit); ok {
								declType = rhsComp.Type
								return false
							}
						}
					}
				}
				return true
			})
			if declType != nil {
				return declType
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

func getASTTypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	if idx, ok := expr.(*ast.IndexExpr); ok {
		return getASTTypeName(idx.X)
	}
	if idxList, ok := expr.(*ast.IndexListExpr); ok {
		return getASTTypeName(idxList.X)
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		return sel.Sel.Name
	}
	return ""
}

func isProvenDBASTType(expr ast.Expr, file *ast.File) bool {
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
				case "DB", "Pool", "Conn", "Tx", "Rows", "Row", "Batch", "Stmt":
					return true
				}
			}
		}
	}
	if id, ok := expr.(*ast.Ident); ok {
		switch id.Name {
		case "DB", "Querier", "DBTX", "Tx", "Pool":
			return true
		}
		if ts := findTypeSpec(id.Name, file); ts != nil {
			return isDBTypeSpec(ts, file)
		}
	}
	return false
}

func findTypeSpec(name string, file *ast.File) *ast.TypeSpec {
	if file == nil || name == "" {
		return nil
	}
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.TYPE {
			for _, spec := range gen.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == name {
					return ts
				}
			}
		}
	}
	return nil
}

func isDBTypeSpec(ts *ast.TypeSpec, file *ast.File) bool {
	if ts == nil {
		return false
	}
	switch t := ts.Type.(type) {
	case *ast.InterfaceType:
		if t.Methods == nil {
			return false
		}
		for _, m := range t.Methods.List {
			for _, name := range m.Names {
				switch name.Name {
				case "Query", "QueryRow", "Exec", "ExecContext", "Begin", "BeginTx":
					return true
				}
			}
		}
	case *ast.StructType:
		if t.Fields == nil {
			return false
		}
		for _, field := range t.Fields.List {
			if isProvenDBASTType(field.Type, file) {
				return true
			}
		}
	}
	return false
}

func isAssignedFromDBConstructor(expr ast.Expr, fn *ast.FuncDecl) bool {
	id, ok := expr.(*ast.Ident)
	if !ok || fn == nil || fn.Body == nil {
		return false
	}
	var isConstructor bool
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if isConstructor {
			return false
		}
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name && i < len(as.Rhs) {
				if call, ok := as.Rhs[i].(*ast.CallExpr); ok && isDBPoolConstructorCall(call) {
					isConstructor = true
					return false
				}
			}
		}
		return true
	})
	return isConstructor
}

func isDBPoolConstructorCall(call *ast.CallExpr) bool {
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
