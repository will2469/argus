// Package a06_runtime_ddl provides declaration position and scope resolution utilities.
package a06_runtime_ddl

import (
	"go/ast"
	"go/token"
)

func findDeclPos(id *ast.Ident, fn *ast.FuncDecl, file *ast.File) token.Pos {
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
					if gen, ok := s.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
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

	return token.NoPos
}

func getEnclosingBlocks(root ast.Node, pos token.Pos) []*ast.BlockStmt {
	if root == nil {
		return nil
	}
	var blocks []*ast.BlockStmt
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil || n.Pos() > pos || n.End() < pos {
			return false
		}
		if b, ok := n.(*ast.BlockStmt); ok {
			blocks = append(blocks, b)
		}
		return true
	})
	return blocks
}
