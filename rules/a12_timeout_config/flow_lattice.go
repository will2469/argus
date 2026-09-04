// Package a12_timeout_config defines lattice operations and scope resolution for data flow analysis.
package a12_timeout_config

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// meetStatus computes the meet operator (infimum) across branching control flow paths.
// All required timeouts must hold along all paths (meet &&).
// If any path introduces a zero timeout, it fails closed (join ||).
func meetStatus(s1, s2 ConfigStatus) ConfigStatus {
	res := ConfigStatus{
		HasStatementTimeout:  s1.HasStatementTimeout && s2.HasStatementTimeout,
		HasLockTimeout:       s1.HasLockTimeout && s2.HasLockTimeout,
		HasIdleInTransaction: s1.HasIdleInTransaction && s2.HasIdleInTransaction,
		HasMaxConnIdleTime:   s1.HasMaxConnIdleTime && s2.HasMaxConnIdleTime,
		HasMaxConnLifetime:   s1.HasMaxConnLifetime && s2.HasMaxConnLifetime,
		HasZeroTimeout:       s1.HasZeroTimeout || s2.HasZeroTimeout,
	}
	if s1.HasZeroTimeout {
		res.ZeroTimeoutParam = s1.ZeroTimeoutParam
	} else if s2.HasZeroTimeout {
		res.ZeroTimeoutParam = s2.ZeroTimeoutParam
	}
	return res
}

func isTerminating(stmt ast.Stmt) bool {
	if stmt == nil {
		return false
	}
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ExprStmt:
		if call, ok := s.X.(*ast.CallExpr); ok {
			fnName := exprToString(call.Fun)
			return fnName == "panic" || strings.HasSuffix(fnName, ".Fatal") || strings.HasSuffix(fnName, ".Fatalf") || fnName == "os.Exit"
		}
	case *ast.BlockStmt:
		if len(s.List) == 0 {
			return false
		}
		return isTerminating(s.List[len(s.List)-1])
	case *ast.IfStmt:
		if s.Else != nil && isTerminating(s.Body) && isTerminating(s.Else) {
			return true
		}
	}
	return false
}

func isSameTarget(pass *analysis.Pass, expr ast.Expr, targetObj types.Object, targetDeclPos token.Pos, body *ast.BlockStmt) bool {
	id := getRootIdent(expr)
	if id == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil && targetObj != nil {
		if obj := pass.TypesInfo.Uses[id]; obj != nil {
			return obj == targetObj
		}
		if obj := pass.TypesInfo.Defs[id]; obj != nil {
			return obj == targetObj
		}
		return false
	}
	if targetDeclPos != token.NoPos && body != nil {
		declPos := findDeclPos(body, id)
		return declPos == targetDeclPos
	}
	return false
}

func findDeclPos(body *ast.BlockStmt, id *ast.Ident) token.Pos {
	if id == nil || body == nil {
		return token.NoPos
	}
	if id.Obj != nil {
		if f, ok := id.Obj.Decl.(*ast.Field); ok {
			return f.Pos()
		}
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
