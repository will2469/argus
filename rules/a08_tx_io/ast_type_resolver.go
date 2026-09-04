// Package a08_tx_io provides AST-level type and constructor resolution
// for environments where type checker information is absent or partial.
package a08_tx_io

import (
	"go/ast"
	"go/token"

	"github.com/will2469/argus/shared/dbident"
)

func findASTType(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) ast.Expr {
	if expr == nil {
		return nil
	}

	if id, ok := expr.(*ast.Ident); ok {
		if fn == nil && file != nil {
			fn = findEnclosingFuncDecl(file, id.Pos())
		}
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
			for _, stmt := range fn.Body.List {
				switch s := stmt.(type) {
				case *ast.DeclStmt:
					if gen, ok := s.Decl.(*ast.GenDecl); ok {
						for _, spec := range gen.Specs {
							if vs, ok := spec.(*ast.ValueSpec); ok {
								for _, name := range vs.Names {
									if name.Name == id.Name {
										return vs.Type
									}
								}
							}
						}
					}
				case *ast.AssignStmt:
					for i, lhs := range s.Lhs {
						if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name {
							if i < len(s.Rhs) {
								if comp, ok := s.Rhs[i].(*ast.CompositeLit); ok {
									return comp.Type
								}
								if ta, ok := s.Rhs[i].(*ast.TypeAssertExpr); ok {
									return ta.Type
								}
								if call, ok := s.Rhs[i].(*ast.CallExpr); ok {
									if ret := resolveCallReturnTypeAST(call, 0, fn, file); ret != nil {
										return ret
									}
								}
							} else if len(s.Rhs) == 1 {
								if call, ok := s.Rhs[0].(*ast.CallExpr); ok {
									if ret := resolveCallReturnTypeAST(call, i, fn, file); ret != nil {
										return ret
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if xID, ok := sel.X.(*ast.Ident); ok {
			xType := findASTType(xID, fn, file)
			structName := dbident.GetASTTypeName(xType)
			if structName != "" && file != nil {
				ts := dbident.FindTypeSpec(structName, file)
				if ts != nil {
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

	return nil
}

func resolveCallReturnTypeAST(call *ast.CallExpr, targetIdx int, fn *ast.FuncDecl, file *ast.File) ast.Expr {
	if call == nil {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	recvType := findASTType(sel.X, fn, file)
	if recvType == nil {
		return nil
	}

	if star, ok := recvType.(*ast.StarExpr); ok {
		recvType = star.X
	}
	if rSel, ok := recvType.(*ast.SelectorExpr); ok {
		if pkgID, ok := rSel.X.(*ast.Ident); ok {
			if dbident.IsImportedDBPackageIdent(file, pkgID.Name) {
				switch rSel.Sel.Name {
				case "DB":
					if (sel.Sel.Name == "Begin" || sel.Sel.Name == "BeginTx") && targetIdx == 0 {
						return &ast.StarExpr{
							X: &ast.SelectorExpr{X: ast.NewIdent(pkgID.Name), Sel: ast.NewIdent("Tx")},
						}
					}
				case "Pool", "Conn":
					if (sel.Sel.Name == "Begin" || sel.Sel.Name == "BeginTx") && targetIdx == 0 {
						return &ast.SelectorExpr{X: ast.NewIdent("pgx"), Sel: ast.NewIdent("Tx")}
					}
				}
			}
		}
	}

	typeName := dbident.GetASTTypeName(recvType)
	if typeName != "" && file != nil {
		ts := dbident.FindTypeSpec(typeName, file)
		if ts != nil {
			if iface, ok := ts.Type.(*ast.InterfaceType); ok && iface.Methods != nil {
				for _, m := range iface.Methods.List {
					for _, name := range m.Names {
						if name.Name == sel.Sel.Name {
							if ft, ok := m.Type.(*ast.FuncType); ok && ft.Results != nil {
								currIdx := 0
								for _, resField := range ft.Results.List {
									if len(resField.Names) == 0 {
										if currIdx == targetIdx {
											return resField.Type
										}
										currIdx++
									} else {
										for range resField.Names {
											if currIdx == targetIdx {
												return resField.Type
											}
											currIdx++
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

func findEnclosingFuncDecl(file *ast.File, pos token.Pos) *ast.FuncDecl {
	if file == nil {
		return nil
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Pos() <= pos && pos <= fn.End() {
				return fn
			}
		}
	}
	return nil
}
