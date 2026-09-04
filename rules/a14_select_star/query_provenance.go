// Package a14_select_star tracks database query argument provenance and resolves SQL strings
// with object-identity precision, preventing shadowing corruption and lexical leakage.
package a14_select_star

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

// isDatabaseCall verifies that a call expression targets a genuine database querier.
func isDatabaseCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil || !callsite.IsDBQueryMethod(sel.Sel.Name) {
		return false
	}

	// 1. Semantic Type Resolution via types.Info
	if pass != nil && pass.TypesInfo != nil {
		if recvType := pass.TypesInfo.TypeOf(sel.X); recvType != nil {
			if callsite.IsPgxOrSQLType(recvType) {
				return true
			}
			tName := strings.ToLower(recvType.String())
			return strings.Contains(tName, "db") || strings.Contains(tName, "pool") ||
				strings.Contains(tName, "tx") || strings.Contains(tName, "querier") ||
				strings.Contains(tName, "repo") || strings.Contains(tName, "store")
		}
	}

	// 2. Syntactic heuristic in standalone mode (pass == nil)
	isDBIdent := func(name string) bool {
		lower := strings.ToLower(name)
		return lower == "q" || strings.Contains(lower, "db") || strings.Contains(lower, "pool") ||
			strings.Contains(lower, "tx") || strings.Contains(lower, "conn") ||
			strings.Contains(lower, "repo") || strings.Contains(lower, "store") ||
			strings.Contains(lower, "dao") || strings.Contains(lower, "queries")
	}

	if id, ok := sel.X.(*ast.Ident); ok {
		return isDBIdent(id.Name)
	}
	if innerSel, ok := sel.X.(*ast.SelectorExpr); ok {
		return isDBIdent(innerSel.Sel.Name)
	}
	return false
}

// ResolveQueryStrings extracts all compile-time SQL query strings passed to a DB call.
func ResolveQueryStrings(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) []string {
	sqlArg := callsite.ExtractSQLArg(call, pass)
	if sqlArg == nil {
		return nil
	}
	return resolveExprStrings(pass, file, sqlArg, call.Pos(), 0)
}

func resolveExprStrings(pass *analysis.Pass, file *ast.File, expr ast.Expr, pos token.Pos, depth int) []string {
	if depth > 5 || expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return []string{strings.Trim(e.Value, "`\"")}
		}
	case *ast.ParenExpr:
		return resolveExprStrings(pass, file, e.X, pos, depth)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return concatStringLists(
				resolveExprStrings(pass, file, e.X, pos, depth+1),
				resolveExprStrings(pass, file, e.Y, pos, depth+1),
			)
		}
	case *ast.Ident:
		if pass != nil && pass.TypesInfo != nil {
			return traceIdentObject(pass, file, e, pos, depth)
		}
		if file != nil {
			return traceIdentASTScope(file, e, pos, depth)
		}
	}
	return nil
}

func traceIdentObject(pass *analysis.Pass, file *ast.File, id *ast.Ident, callPos token.Pos, depth int) []string {
	obj := pass.TypesInfo.Uses[id]
	if obj == nil {
		obj = pass.TypesInfo.Defs[id]
	}
	if obj == nil {
		return nil
	}
	if c, ok := obj.(*types.Const); ok {
		return []string{strings.Trim(c.Val().ExactString(), "`\"")}
	}

	var results []string
	var root ast.Node = file
	if enclosing := findEnclosingFunc(file, callPos); enclosing != nil {
		root = enclosing
	}

	ast.Inspect(root, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			if stmt.Pos() >= callPos {
				return true
			}
			for i, lhs := range stmt.Lhs {
				lhsIdent, ok := lhs.(*ast.Ident)
				if !ok || i >= len(stmt.Rhs) {
					continue
				}
				lhsObj := pass.TypesInfo.Defs[lhsIdent]
				if lhsObj == nil {
					lhsObj = pass.TypesInfo.Uses[lhsIdent]
				}
				if lhsObj == obj {
					sub := resolveExprStrings(pass, file, stmt.Rhs[i], stmt.Pos(), depth+1)
					results = append(results, sub...)
				}
			}
		case *ast.ValueSpec:
			for i, name := range stmt.Names {
				if i < len(stmt.Values) && pass.TypesInfo.Defs[name] == obj {
					sub := resolveExprStrings(pass, file, stmt.Values[i], stmt.Pos(), depth+1)
					results = append(results, sub...)
				}
			}
		}
		return true
	})
	return results
}

func traceIdentASTScope(file *ast.File, id *ast.Ident, callPos token.Pos, depth int) []string {
	blocks := getEnclosingBlocks(file, callPos)
	for i := len(blocks) - 1; i >= 0; i-- {
		b := blocks[i]
		var blockResults []string
		shadowedInBlock := false

		for _, stmt := range b.List {
			if stmt.Pos() >= callPos {
				continue
			}
			assign, ok := stmt.(*ast.AssignStmt)
			if !ok {
				continue
			}
			for idx, lhs := range assign.Lhs {
				lhsId, ok := lhs.(*ast.Ident)
				if ok && lhsId.Name == id.Name && idx < len(assign.Rhs) {
					if assign.Tok == token.DEFINE {
						shadowedInBlock = true
					}
					sub := resolveExprStrings(nil, file, assign.Rhs[idx], assign.Pos(), depth+1)
					blockResults = append(blockResults, sub...)
				}
			}
		}
		if len(blockResults) > 0 || shadowedInBlock {
			return blockResults
		}
	}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			valSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range valSpec.Names {
				if name.Name == id.Name && i < len(valSpec.Values) {
					return resolveExprStrings(nil, file, valSpec.Values[i], valSpec.Pos(), depth+1)
				}
			}
		}
	}
	return nil
}

func concatStringLists(left, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	var res []string
	for _, l := range left {
		for _, r := range right {
			res = append(res, l+r)
		}
	}
	return res
}

func getEnclosingBlocks(root ast.Node, pos token.Pos) []*ast.BlockStmt {
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

func findEnclosingFunc(file *ast.File, pos token.Pos) *ast.FuncDecl {
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
