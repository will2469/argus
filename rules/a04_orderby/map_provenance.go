// Package a04_orderby verifies map provenance to ensure map lookups in ORDER BY
// clauses originate from static, closed-set allowlist maps with constant string values.
package a04_orderby

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// isAllowlistMapLookup checks whether an IndexExpr (map[key]) retrieves from a verified allowlist map.
func isAllowlistMapLookup(idxExpr *ast.IndexExpr, body *ast.BlockStmt, file *ast.File, pass *analysis.Pass, allowedCols []string) bool {
	if idxExpr == nil {
		return false
	}

	// 1. Direct composite literal map lookup: map[string]string{"a": "col_a"}[userSort]
	if compLit, ok := idxExpr.X.(*ast.CompositeLit); ok {
		return isConstantStringMapLit(compLit, allowedCols)
	}

	// 2. Map identifier lookup: sortMap[userSort]
	ident, ok := idxExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	// A. Check local function body definitions
	if body != nil {
		if mapLit, isLocal := findLocalMapLiteral(ident.Name, body); isLocal {
			if isConstantStringMapLit(mapLit, allowedCols) && !isMapDynamicallyMutated(ident.Name, body) {
				return true
			}
		}
	}

	// B. Check package-level map declarations in the current file
	if file != nil {
		if mapLit, isPkg := findPackageMapLiteral(ident.Name, file); isPkg {
			return isConstantStringMapLit(mapLit, allowedCols)
		}
	}

	// C. Check package sibling files when pass is available
	if pass != nil {
		for _, f := range pass.Files {
			if f == file {
				continue
			}
			if mapLit, isPkg := findPackageMapLiteral(ident.Name, f); isPkg {
				return isConstantStringMapLit(mapLit, allowedCols)
			}
		}
	}

	return false
}

// isConstantStringMapLit verifies that all values in a map composite literal are compile-time string constants.
func isConstantStringMapLit(lit *ast.CompositeLit, allowedCols []string) bool {
	if lit == nil || len(lit.Elts) == 0 {
		return false
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return false
		}
		// The mapped column value must be a constant string literal
		valLit, isLit := kv.Value.(*ast.BasicLit)
		if !isLit || valLit.Kind != token.STRING {
			return false
		}
		if len(allowedCols) > 0 {
			unquoted := unquoteString(valLit.Value)
			if !isColAllowed(unquoted, allowedCols) {
				return false
			}
		}
	}
	return true
}

func findLocalMapLiteral(varName string, body *ast.BlockStmt) (*ast.CompositeLit, bool) {
	var foundLit *ast.CompositeLit
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == varName {
					if i < len(s.Rhs) {
						if comp, ok := s.Rhs[i].(*ast.CompositeLit); ok {
							foundLit = comp
							return false
						}
					}
				}
			}
		case *ast.DeclStmt:
			if gen, ok := s.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				for _, spec := range gen.Specs {
					if valSpec, ok := spec.(*ast.ValueSpec); ok {
						for i, name := range valSpec.Names {
							if name.Name == varName && i < len(valSpec.Values) {
								if comp, ok := valSpec.Values[i].(*ast.CompositeLit); ok {
									foundLit = comp
									return false
								}
							}
						}
					}
				}
			}
		}
		return true
	})
	return foundLit, foundLit != nil
}

func findPackageMapLiteral(varName string, file *ast.File) (*ast.CompositeLit, bool) {
	if file == nil {
		return nil, false
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			valSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range valSpec.Names {
				if name.Name == varName && i < len(valSpec.Values) {
					if comp, ok := valSpec.Values[i].(*ast.CompositeLit); ok {
						return comp, true
					}
				}
			}
		}
	}
	return nil, false
}

func isMapDynamicallyMutated(varName string, body *ast.BlockStmt) bool {
	mutated := false
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range assign.Lhs {
			if idx, ok := lhs.(*ast.IndexExpr); ok {
				if id, ok := idx.X.(*ast.Ident); ok && id.Name == varName {
					mutated = true
					return false
				}
			}
		}
		return true
	})
	return mutated
}

func unquoteString(s string) string {
	unquoted, err := strconv.Unquote(s)
	if err != nil {
		return strings.Trim(s, "`\"")
	}
	return unquoted
}

func isColAllowed(col string, allowedCols []string) bool {
	col = strings.ToLower(strings.TrimSpace(col))
	for _, allowed := range allowedCols {
		if strings.EqualFold(col, strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}
