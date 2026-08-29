// Package a18_rows_err implements the ARGUS-A18 static analysis rule.
package a18_rows_err

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// Issue represents a diagnostic finding for ARGUS-A18.
type Issue struct {
	Pos     token.Pos
	Message string
}

// ExtractRowsLoop examines an ast.ForStmt to see if it iterates over a database rows cursor (.Next()).
func ExtractRowsLoop(pass *analysis.Pass, forStmt *ast.ForStmt) (string, bool) {
	if forStmt == nil || forStmt.Cond == nil {
		return "", false
	}

	call, ok := forStmt.Cond.(*ast.CallExpr)
	if !ok {
		return "", false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Next" {
		return "", false
	}

	if !IsDatabaseRowsReceiver(pass, sel.X) {
		return "", false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}

	return ident.Name, true
}

// InspectFile inspects an entire AST file for unverified rows cursor loops.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap) []Issue {
	if file == nil {
		return nil
	}

	// If file does not import database packages and typesInfo is not available, skip non-db files
	if pass == nil || pass.TypesInfo == nil {
		if !HasDatabaseImports(file) {
			return nil
		}
	}

	var issues []Issue
	ast.Inspect(file, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}
		blockIssues := InspectBlock(pass, fset, block, dm)
		issues = append(issues, blockIssues...)
		return true
	})

	return issues
}

// InspectBlock checks a BlockStmt for unverified rows cursor loops.
func InspectBlock(pass *analysis.Pass, fset *token.FileSet, block *ast.BlockStmt, dm *directives.DirectiveMap) []Issue {
	if block == nil {
		return nil
	}

	var issues []Issue

	for i, stmt := range block.List {
		forStmt, ok := stmt.(*ast.ForStmt)
		if !ok {
			continue
		}

		rowsVar, isRowsLoop := ExtractRowsLoop(pass, forStmt)
		if !isRowsLoop {
			continue
		}

		if dm != nil && dm.IsIgnored(fset, forStmt.Pos(), RuleCode) {
			continue
		}

		remainingStmts := block.List[i+1:]
		if !HasValidPostLoopErrCheck(remainingStmts, rowsVar) {
			msg := fmt.Sprintf("[%s] missing %s.Err() check after for %s.Next() loop; unchecked error risks silent dataset truncation on network drop or statement timeout (CWE-391)",
				RuleCode, rowsVar, rowsVar)
			issues = append(issues, Issue{
				Pos:     forStmt.Pos(),
				Message: msg,
			})
		}
	}

	return issues
}
