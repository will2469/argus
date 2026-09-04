// Package a09_advisory_lock provides lexical object identity and scope resolution
// preventing variable shadowing from breaking static analysis proofs.
package a09_advisory_lock

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type varKey struct {
	obj     types.Object
	declPos token.Pos
	name    string
}

func makeVarKey(pass *analysis.Pass, id *ast.Ident, body *ast.BlockStmt) varKey {
	if id == nil {
		return varKey{}
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Defs[id]; obj != nil {
			return varKey{obj: obj, name: id.Name}
		}
		if obj := pass.TypesInfo.Uses[id]; obj != nil {
			return varKey{obj: obj, name: id.Name}
		}
		if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
			return varKey{obj: obj, name: id.Name}
		}
	}
	declPos := findDeclPos(body, id)
	return varKey{declPos: declPos, name: id.Name}
}

func findDeclPos(body *ast.BlockStmt, id *ast.Ident) token.Pos {
	if id == nil || body == nil {
		return token.NoPos
	}
	blocks := getEnclosingBlocks(body, id.Pos())
	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		for _, stmt := range b.List {
			switch s := stmt.(type) {
			case *ast.AssignStmt:
				if s.Tok == token.DEFINE {
					for _, lhs := range s.Lhs {
						if ident, ok := lhs.(*ast.Ident); ok {
							if ident == id {
								return ident.Pos()
							}
							if stmt.Pos() < id.Pos() && ident.Name == id.Name {
								return ident.Pos()
							}
						}
					}
				}
			case *ast.DeclStmt:
				if gen, ok := s.Decl.(*ast.GenDecl); ok {
					for _, spec := range gen.Specs {
						if valSpec, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range valSpec.Names {
								if name == id {
									return name.Pos()
								}
								if stmt.Pos() < id.Pos() && name.Name == id.Name {
									return name.Pos()
								}
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
	ast.Inspect(root, func(n ast.Node) bool {
		if b, ok := n.(*ast.BlockStmt); ok {
			if b.Pos() <= target && target <= b.End() {
				blocks = append(blocks, b)
			}
		}
		return true
	})

	// Sort from outermost (largest span) to innermost (smallest span)
	for i := 0; i < len(blocks); i++ {
		for j := i + 1; j < len(blocks); j++ {
			spanI := blocks[i].End() - blocks[i].Pos()
			spanJ := blocks[j].End() - blocks[j].Pos()
			if spanJ > spanI {
				blocks[i], blocks[j] = blocks[j], blocks[i]
			}
		}
	}
	return blocks
}

// isSameObject verifies whether two identifier AST nodes refer to the exact same types.Object or declaration position,
// strictly rejecting lexical name-only fallback to prevent shadowing bugs.
func isSameObject(pass *analysis.Pass, lhsID, targetID *ast.Ident, body *ast.BlockStmt) bool {
	if lhsID == nil || targetID == nil {
		return false
	}
	k1 := makeVarKey(pass, lhsID, body)
	k2 := makeVarKey(pass, targetID, body)
	if k1.obj != nil && k2.obj != nil {
		return k1.obj == k2.obj
	}
	if k1.declPos != token.NoPos && k2.declPos != token.NoPos {
		return k1.declPos == k2.declPos
	}
	return false
}

// resolveStringConstant extracts the constant string value if the identifier refers to a string constant.
func resolveStringConstant(pass *analysis.Pass, id *ast.Ident) (string, bool) {
	if id == nil {
		return "", false
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
			if c, ok := obj.(*types.Const); ok && c.Val().Kind() == constant.String {
				return constant.StringVal(c.Val()), true
			}
		}
	}
	if id.Obj != nil && id.Obj.Kind == ast.Con {
		if vs, ok := id.Obj.Decl.(*ast.ValueSpec); ok {
			for _, val := range vs.Values {
				if lit, ok := val.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					return strings.Trim(lit.Value, "`\""), true
				}
			}
		}
	}
	return "", false
}
