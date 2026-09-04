// Package a08_tx_io provides declaration position and scope resolution for variables.
package a08_tx_io

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
					for _, lhs := range s.Lhs {
						if ident, ok := lhs.(*ast.Ident); ok && ident.Name == id.Name {
							return ident.Pos()
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
