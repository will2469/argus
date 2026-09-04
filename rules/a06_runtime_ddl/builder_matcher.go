// Package a06_runtime_ddl provides semantic method recognition for string builders
// (strings.Builder and bytes.Buffer), preventing lexical selector-name spoofing.
package a06_runtime_ddl

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// isSemanticBuilderCall checks if call is a method call on *strings.Builder or *bytes.Buffer.
func isSemanticBuilderCall(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, call *ast.CallExpr) (method string, targetIdent *ast.Ident, ok bool) {
	if call == nil {
		return "", nil, false
	}
	sel, isSel := call.Fun.(*ast.SelectorExpr)
	if !isSel {
		return "", nil, false
	}

	targetIdent = getRootIdent(sel.X)
	if targetIdent == nil {
		return "", nil, false
	}

	// 1. Pass mode with types.Info (exact type verification)
	if pass != nil && pass.TypesInfo != nil {
		if selType, okSel := pass.TypesInfo.Selections[sel]; okSel {
			if isStringBuilderType(selType.Recv()) {
				if methodObj := selType.Obj(); methodObj != nil && methodObj.Pkg() != nil {
					pkg := methodObj.Pkg().Path()
					if pkg == "strings" || pkg == "bytes" {
						return sel.Sel.Name, targetIdent, true
					}
				}
			}
		}
		if tv, okTv := pass.TypesInfo.Types[sel.X]; okTv && tv.Type != nil {
			if isStringBuilderType(tv.Type) {
				return sel.Sel.Name, targetIdent, true
			}
		}
	}

	// 2. Standalone mode (pass == nil) - exact AST type verification only
	if isDeclaredAsBuilder(file, fn, targetIdent) {
		return sel.Sel.Name, targetIdent, true
	}

	return "", nil, false
}

func getRootIdent(expr ast.Expr) *ast.Ident {
	switch e := expr.(type) {
	case *ast.Ident:
		return e
	case *ast.UnaryExpr:
		return getRootIdent(e.X)
	case *ast.ParenExpr:
		return getRootIdent(e.X)
	}
	return nil
}

func isDeclaredAsBuilder(file *ast.File, fn *ast.FuncDecl, id *ast.Ident) bool {
	if id == nil {
		return false
	}

	// Fail-closed: only accept EXACT type matches, not heuristic name patterns
	// We only check for proven *strings.Builder or *bytes.Buffer declarations

	if fn != nil && fn.Body != nil {
		blocks := getEnclosingBlocks(fn.Body, id.Pos())
		for i := len(blocks) - 1; i >= 0; i-- {
			b := blocks[i]
			for _, stmt := range b.List {
				if stmt.Pos() >= id.Pos() {
					continue
				}
				switch s := stmt.(type) {
				case *ast.DeclStmt:
					if gen, ok := s.Decl.(*ast.GenDecl); ok {
						for _, spec := range gen.Specs {
							if valSpec, ok := spec.(*ast.ValueSpec); ok {
								for _, name := range valSpec.Names {
									if name.Name == id.Name && isBuilderTypeExpr(valSpec.Type) {
										return true
									}
								}
							}
						}
					}
				case *ast.AssignStmt:
					for idx, lhs := range s.Lhs {
						if ident, ok := lhs.(*ast.Ident); ok && ident.Name == id.Name && idx < len(s.Rhs) {
							if isBuilderExprAST(s.Rhs[idx]) {
								return true
							}
						}
					}
				}
			}
		}

		if fn.Type != nil && fn.Type.Params != nil {
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					if name.Name == id.Name && isBuilderTypeExpr(field.Type) {
						return true
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
							if name.Name == id.Name && isBuilderTypeExpr(valSpec.Type) {
								return true
							}
						}
					}
				}
			}
		}
	}

	// Fail-closed: removed weak heuristic fallbacks (sb, builder, *builder suffix)
	// Only accept proven *strings.Builder or *bytes.Buffer types
	return false
}

func isBuilderTypeExpr(t ast.Expr) bool {
	if t == nil {
		return false
	}
	switch e := t.(type) {
	case *ast.StarExpr:
		return isBuilderTypeExpr(e.X)
	case *ast.SelectorExpr:
		if id, ok := e.X.(*ast.Ident); ok {
			if (id.Name == "strings" && e.Sel.Name == "Builder") ||
				(id.Name == "bytes" && e.Sel.Name == "Buffer") {
				return true
			}
		}
	}
	return false
}

func isBuilderExprAST(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		return isBuilderExprAST(e.X)
	case *ast.CompositeLit:
		return isBuilderTypeExpr(e.Type)
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewBuffer" {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "bytes" {
				return true
			}
		}
	}
	return false
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
