// Package a04_orderby verifies sort direction variables ("ASC" / "DESC") across all execution paths.
package a04_orderby

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// IsSortDirectionSafe verifies that a direction variable strictly evaluates to "ASC" or "DESC"
// along all reachable execution paths.
func IsSortDirectionSafe(ident *ast.Ident, body *ast.BlockStmt) bool {
	if ident == nil || body == nil {
		return false
	}

	var hasSafeAssignment bool
	var hasUnsafeAssignment bool
	var hasSafeInit bool
	var ifWithBothBranchesSafe bool

	ast.Inspect(body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range stmt.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name != ident.Name || i >= len(stmt.Rhs) {
					continue
				}
				rhs := stmt.Rhs[i]
				if isDirectionLit(rhs) {
					hasSafeAssignment = true
					if stmt.Tok == token.DEFINE {
						hasSafeInit = true
					}
				} else {
					hasUnsafeAssignment = true
				}
			}

		case *ast.IfStmt:
			if hasBothBranchesSafeAssign(stmt, ident.Name) {
				ifWithBothBranchesSafe = true
			}
		}
		return true
	})

	if hasUnsafeAssignment || !hasSafeAssignment {
		return false
	}

	return hasSafeInit || ifWithBothBranchesSafe
}

func isDirectionLit(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	val, err := strconv.Unquote(lit.Value)
	if err != nil {
		val = strings.Trim(lit.Value, "`\"")
	}
	u := strings.ToUpper(strings.TrimSpace(val))
	return u == "ASC" || u == "DESC"
}

func hasBothBranchesSafeAssign(ifStmt *ast.IfStmt, varName string) bool {
	if ifStmt.Body == nil || ifStmt.Else == nil {
		return false
	}
	ifSafe := blockAssignsSafeDirection(ifStmt.Body, varName)
	var elseSafe bool
	switch el := ifStmt.Else.(type) {
	case *ast.BlockStmt:
		elseSafe = blockAssignsSafeDirection(el, varName)
	case *ast.IfStmt:
		elseSafe = hasBothBranchesSafeAssign(el, varName)
	}
	return ifSafe && elseSafe
}

func blockAssignsSafeDirection(block *ast.BlockStmt, varName string) bool {
	if block == nil {
		return false
	}
	for _, s := range block.List {
		assign, ok := s.(*ast.AssignStmt)
		if !ok {
			continue
		}
		for i, lhs := range assign.Lhs {
			id, ok := lhs.(*ast.Ident)
			if ok && id.Name == varName && i < len(assign.Rhs) {
				if isDirectionLit(assign.Rhs[i]) {
					return true
				}
			}
		}
	}
	return false
}
