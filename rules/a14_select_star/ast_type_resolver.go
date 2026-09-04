// Package a14_select_star provides AST-level type and constructor resolution
// for environments where type checker information is absent or partial.
package a14_select_star

import (
	"go/ast"

	"github.com/will2469/argus/shared/dbident"
)

func findASTType(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) ast.Expr {
	if expr == nil {
		return nil
	}

	if id, ok := expr.(*ast.Ident); ok {
		if fn != nil {
			fieldLists := []*ast.FieldList{fn.Recv}
			if fn.Type != nil {
				fieldLists = append(fieldLists, fn.Type.Params)
			}
			for _, fl := range fieldLists {
				if fl != nil {
					for _, field := range fl.List {
						for _, name := range field.Names {
							if name.Name == id.Name {
								return field.Type
							}
						}
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
			structName := dbident.GetASTTypeName(xType)
			if ts := dbident.FindTypeSpec(structName, file); ts != nil {
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

	return nil
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
				case "DB", "Pool", "Conn", "Tx", "Rows", "Row", "Batch", "Stmt", "Result", "CommandTag":
					return true
				}
			}
		}
	}
	if id, ok := expr.(*ast.Ident); ok {
		if ts := dbident.FindTypeSpec(id.Name, file); ts != nil {
			return isDBTypeSpec(ts, file)
		}
	}
	return false
}

func isDBTypeSpec(ts *ast.TypeSpec, _ *ast.File) bool {
	if ts == nil {
		return false
	}
	switch t := ts.Type.(type) {
	case *ast.InterfaceType:
		if t.Methods == nil {
			return false
		}
		for _, m := range t.Methods.List {
			if ft, ok := m.Type.(*ast.FuncType); ok && hasDBDriverMethodAST(ft) {
				return true
			}
		}
	}
	return false
}

func hasDBDriverMethodAST(ft *ast.FuncType) bool {
	if ft == nil || ft.Results == nil {
		return false
	}
	for _, f := range ft.Results.List {
		typ := f.Type
		if star, ok := typ.(*ast.StarExpr); ok {
			typ = star.X
		}
		if sel, ok := typ.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				switch id.Name {
				case "sql", "pgx", "pgconn", "sqlx", "pq":
					return true
				}
			}
		}
	}
	return false
}

func isAssignedFromDBConstructor(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) bool {
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
				if call, ok := as.Rhs[i].(*ast.CallExpr); ok && dbident.IsDBPoolConstructorCall(call, file) {
					isConstructor = true
					return false
				}
			}
		}
		return true
	})
	return isConstructor
}
