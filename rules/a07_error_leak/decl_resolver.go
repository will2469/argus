// Package a07_error_leak provides scope and declaration lookup for variables and identifiers.
package a07_error_leak

import (
	"go/ast"
	"go/token"
)

func findDeclPos(file *ast.File, fn *ast.FuncDecl, id *ast.Ident) token.Pos {
	if id == nil {
		return token.NoPos
	}
	if fn != nil && fn.Body != nil {
		blocks := getEnclosingBlocks(fn.Body, id.Pos())
		for i := len(blocks) - 1; i >= 0; i-- {
			b := blocks[i]
			for _, stmt := range b.List {
				if stmt.Pos() >= id.Pos() {
					continue
				}
				switch s := stmt.(type) {
				case *ast.AssignStmt:
					if s.Tok == token.DEFINE {
						for _, lhs := range s.Lhs {
							if ident, ok := lhs.(*ast.Ident); ok && ident.Name == id.Name {
								return ident.Pos()
							}
						}
					}
				case *ast.DeclStmt:
					if gen, ok := s.Decl.(*ast.GenDecl); ok {
						for _, spec := range gen.Specs {
							if valSpec, ok := spec.(*ast.ValueSpec); ok {
								for _, name := range valSpec.Names {
									if name.Name == id.Name {
										return name.Pos()
									}
								}
							}
						}
					}
				}
			}
		}

		if fn.Type != nil && fn.Type.Params != nil {
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					if name.Name == id.Name {
						return name.Pos()
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
								return name.Pos()
							}
						}
					}
				}
			}
		}
	}

	return id.Pos()
}

func getEnclosingBlocks(root *ast.BlockStmt, target token.Pos) []*ast.BlockStmt {
	if root == nil {
		return nil
	}
	var blocks []*ast.BlockStmt
	var walk func(b *ast.BlockStmt)
	walk = func(b *ast.BlockStmt) {
		if b.Pos() <= target && target <= b.End() {
			blocks = append(blocks, b)
			for _, s := range b.List {
				switch st := s.(type) {
				case *ast.BlockStmt:
					walk(st)
				case *ast.IfStmt:
					if st.Body != nil {
						walk(st.Body)
					}
					if el, ok := st.Else.(*ast.BlockStmt); ok {
						walk(el)
					}
				case *ast.ForStmt:
					if st.Body != nil {
						walk(st.Body)
					}
				case *ast.RangeStmt:
					if st.Body != nil {
						walk(st.Body)
					}
				}
			}
		}
	}
	walk(root)
	return blocks
}

// findASTType resolves the declared AST type for an expression within a function and file.
func findASTType(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) ast.Expr {
	if expr == nil {
		return nil
	}

	if id, ok := expr.(*ast.Ident); ok {
		if fn != nil {
			var fieldLists []*ast.FieldList
			if fn.Recv != nil {
				fieldLists = append(fieldLists, fn.Recv)
			}
			if fn.Type != nil && fn.Type.Params != nil {
				fieldLists = append(fieldLists, fn.Type.Params)
			}
			for _, fl := range fieldLists {
				for _, field := range fl.List {
					for _, name := range field.Names {
						if name.Name == id.Name {
							return field.Type
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
							if typeAssert, ok := s.Rhs[i].(*ast.TypeAssertExpr); ok {
								declType = typeAssert.Type
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
			if ts := findTypeSpec(structName, file); ts != nil {
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

// getASTTypeName extracts the base type name from an AST type expression.
func getASTTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return getASTTypeName(e.X)
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		return getASTTypeName(e.X)
	case *ast.IndexListExpr:
		return getASTTypeName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

// findTypeSpec searches the file for a type declaration matching name.
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

