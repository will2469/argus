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

func isSameTarget(pass *analysis.Pass, expr ast.Expr, targetObj types.Object, varName string) bool {
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
	return id.Name == varName
}

func findDominatingBlock(body *ast.BlockStmt, pos token.Pos, varName string) *ast.BlockStmt {
	var dominating *ast.BlockStmt
	ast.Inspect(body, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok || block.Pos() > pos || pos > block.End() {
			return true
		}
		for _, stmt := range block.List {
			if stmt.Pos() >= pos {
				break
			}
			if assign, ok := stmt.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
				for _, lhs := range assign.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name == varName {
						dominating = block
						return true
					}
				}
			}
			if decl, ok := stmt.(*ast.DeclStmt); ok {
				if gen, ok := decl.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
					for _, spec := range gen.Specs {
						if valSpec, ok := spec.(*ast.ValueSpec); ok {
							for _, id := range valSpec.Names {
								if id.Name == varName {
									dominating = block
									return true
								}
							}
						}
					}
				}
			}
		}
		return true
	})
	if dominating != nil {
		return dominating
	}
	return body
}
