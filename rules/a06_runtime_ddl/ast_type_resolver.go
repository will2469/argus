// Package a06_runtime_ddl provides AST-level type and constructor resolution
// for environments where type checker information is absent or partial.
package a06_runtime_ddl

import (
	"go/ast"
	"go/token"

	"github.com/will2469/argus/shared/dbident"
)

func findASTType(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) ast.Expr {
	if expr == nil {
		return nil
	}

	if id, ok := expr.(*ast.Ident); ok {
		if fn != nil {
			for _, fl := range []*ast.FieldList{fn.Recv, fnParams(fn)} {
				if fl == nil {
					continue
				}
				for _, field := range fl.List {
					for _, name := range field.Names {
						if name.Name == id.Name {
							return field.Type
						}
					}
				}
			}
		}
		if fn != nil && fn.Body != nil {
			var declType ast.Expr
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if declType != nil {
					return false
				}
				switch s := n.(type) {
				case *ast.ValueSpec:
					for _, nm := range s.Names {
						if nm.Name == id.Name && s.Type != nil {
							declType = s.Type
							return false
						}
					}
				case *ast.AssignStmt:
					for i, lhs := range s.Lhs {
						if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name && i < len(s.Rhs) {
							if rhsComp, ok := s.Rhs[i].(*ast.CompositeLit); ok {
								declType = rhsComp.Type
								return false
							}
							if typeAssert, ok := s.Rhs[i].(*ast.TypeAssertExpr); ok {
								declType = typeAssert.Type
								return false
							}
						}
					}
				}
				return true
			})
			if declType != nil {
				return declType
			}
		}
	}

	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if xID, ok := sel.X.(*ast.Ident); ok {
			xType := findASTType(xID, fn, file)
			structName := getASTTypeName(xType)
			if ts := findTypeSpec(structName, file); ts != nil {
				if st, ok := ts.Type.(*ast.StructType); ok && st.Fields != nil {
					for _, f := range st.Fields.List {
						for _, fnm := range f.Names {
							if fnm.Name == sel.Sel.Name {
								return f.Type
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func fnParams(fn *ast.FuncDecl) *ast.FieldList {
	if fn != nil && fn.Type != nil {
		return fn.Type.Params
	}
	return nil
}

func getASTTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return getASTTypeName(e.X)
	case *ast.Ident:
		return e.Name
	case *ast.IndexExpr:
		return getASTTypeName(e.X)
	case *ast.IndexListExpr:
		return getASTTypeName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

func isProvenDBASTType(expr ast.Expr, file *ast.File) bool {
	if expr == nil || file == nil {
		return false
	}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if pkgID, ok := sel.X.(*ast.Ident); ok {
			if dbident.IsImportedDBPackageIdent(file, pkgID.Name) {
				switch sel.Sel.Name {
				case "DB", "Pool", "Conn", "Tx", "Rows", "Row", "Batch", "BatchResults", "Stmt", "Result", "CommandTag":
					return true
				}
			}
		}
	}
	if id, ok := expr.(*ast.Ident); ok {
		return isDBTypeSpec(findTypeSpec(id.Name, file), file)
	}
	return false
}

func findTypeSpec(name string, file *ast.File) *ast.TypeSpec {
	if file == nil || name == "" {
		return nil
	}
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.TYPE {
			for _, spec := range gen.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == name {
					return ts
				}
			}
		}
	}
	return nil
}

func isDBTypeSpec(ts *ast.TypeSpec, file *ast.File) bool {
	if ts == nil || file == nil || !dbident.HasDatabaseImports(file) {
		return false
	}
	switch t := ts.Type.(type) {
	case *ast.InterfaceType:
		if t.Methods == nil {
			return false
		}
		if dbident.HasNonDBStructImplementation(file, t) {
			return false
		}
		hasExec, hasQuery := false, false
		for _, m := range t.Methods.List {
			if _, ok := m.Type.(*ast.FuncType); !ok {
				continue
			}
			for _, nm := range m.Names {
				switch nm.Name {
				case "Exec", "ExecContext":
					if dbident.IsASTExecOrQueryMethod(m, file) {
						hasExec = true
					}
				case "Query", "QueryContext", "QueryRow", "QueryRowContext":
					if dbident.IsASTExecOrQueryMethod(m, file) {
						hasQuery = true
					}
				}
			}
		}
		return hasExec && hasQuery
	}
	return false
}

func isAssignedFromDBConstructor(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) bool {
	id, ok := expr.(*ast.Ident)
	if !ok || fn == nil || fn.Body == nil {
		return false
	}
	var isConstructor bool
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if isConstructor {
			return false
		}
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name && i < len(as.Rhs) {
				if call, ok := as.Rhs[i].(*ast.CallExpr); ok && dbident.IsDBPoolConstructorCall(call, file) {
					isConstructor = true
					return false
				}
			}
		}
		return true
	})
	return isConstructor
}
