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
	case *ast.CallExpr:
		if isFmtSprintf(e) && len(e.Args) > 0 {
			return resolveExprStrings(pass, file, e.Args[0], pos, depth+1)
		}
	case *ast.Ident:
		if pass != nil && pass.TypesInfo != nil {
			obj := pass.TypesInfo.Uses[e]
			if obj == nil {
				obj = pass.TypesInfo.Defs[e]
			}
			if c, ok := obj.(*types.Const); ok {
				return []string{strings.Trim(c.Val().ExactString(), "`\"")}
			}
		}
		var results []string
		for _, def := range getIdentDefExprs(pass, file, e, pos) {
			sub := resolveExprStrings(pass, file, def, pos, depth+1)
			results = append(results, sub...)
		}
		return results
	}
	return nil
}

func getIdentDefExprs(pass *analysis.Pass, file *ast.File, id *ast.Ident, callPos token.Pos) []ast.Expr {
	if id == nil {
		return nil
	}
	// 1. Pass mode with types.Info
	if pass != nil && pass.TypesInfo != nil {
		obj := pass.TypesInfo.Uses[id]
		if obj == nil {
			obj = pass.TypesInfo.Defs[id]
		}
		if obj == nil {
			return nil
		}

		// Package-level variable: inspect pass.Files
		if obj.Parent() == pass.Pkg.Scope() {
			var exprs []ast.Expr
			for _, f := range pass.Files {
				for _, decl := range f.Decls {
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
							if pass.TypesInfo.Defs[name] == obj && i < len(valSpec.Values) {
								exprs = append(exprs, valSpec.Values[i])
							}
						}
					}
				}
			}
			return exprs
		}

		// Local variable: inspect enclosing function
		var exprs []ast.Expr
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
						exprs = append(exprs, stmt.Rhs[i])
					}
				}
			case *ast.ValueSpec:
				for i, name := range stmt.Names {
					if i < len(stmt.Values) && pass.TypesInfo.Defs[name] == obj {
						exprs = append(exprs, stmt.Values[i])
					}
				}
			}
			return true
		})
		return exprs
	}

	// 2. Standalone mode (pass == nil): lexical block scope traversal
	if file != nil {
		blocks := getEnclosingBlocks(file, callPos)
		for i := len(blocks) - 1; i >= 0; i-- {
			b := blocks[i]
			var blockExprs []ast.Expr
			shadowed := false

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
							shadowed = true
						}
						blockExprs = append(blockExprs, assign.Rhs[idx])
					}
				}
			}
			if len(blockExprs) > 0 || shadowed {
				return blockExprs
			}
		}

		// Package-level fallback in file.Decls
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
						return []ast.Expr{valSpec.Values[i]}
					}
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
