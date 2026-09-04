// Package a09_advisory_lock provides AST-level type inspection and method contract
// verification for advisory lock helper receivers when compiler types are absent or partial.
package a09_advisory_lock

import (
	"go/ast"
	"go/token"
	"strings"
)

func findASTReceiverType(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) ast.Expr {
	if expr == nil {
		return nil
	}
	id, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}

	// 1. Check enclosing function parameters
	if fn != nil && fn.Type != nil && fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				if name.Name == id.Name {
					return field.Type
				}
			}
		}
	}

	// 2. Check enclosing function receiver
	if fn != nil && fn.Recv != nil {
		for _, field := range fn.Recv.List {
			for _, name := range field.Names {
				if name.Name == id.Name {
					return field.Type
				}
			}
		}
	}

	// 3. Check enclosing function local declarations
	if fn != nil && fn.Body != nil {
		for _, stmt := range fn.Body.List {
			switch s := stmt.(type) {
			case *ast.DeclStmt:
				if gen, ok := s.Decl.(*ast.GenDecl); ok {
					for _, spec := range gen.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range vs.Names {
								if name.Name == id.Name {
									return vs.Type
								}
							}
						}
					}
				}
			case *ast.AssignStmt:
				for i, lhs := range s.Lhs {
					if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name {
						if i < len(s.Rhs) {
							if comp, ok := s.Rhs[i].(*ast.CompositeLit); ok {
								return comp.Type
							}
							if u, ok := s.Rhs[i].(*ast.UnaryExpr); ok && u.Op == token.AND {
								if comp, ok := u.X.(*ast.CompositeLit); ok {
									return comp.Type
								}
							}
						}
					}
				}
			}
		}
	}

	// 4. Check package-level variables in the file (e.g. var argus Helper)
	if file != nil {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
				for _, spec := range gen.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range vs.Names {
							if name.Name == id.Name {
								return vs.Type
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func hasASTProvenLockMethod(typeName string, methodName string, file *ast.File) bool {
	if file == nil || typeName == "" {
		return false
	}

	// 1. Check method declarations on type: func (recv Type) Method(...)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Name.Name == methodName {
			for _, field := range fn.Recv.List {
				recvName := getASTTypeName(field.Type)
				if recvName == typeName {
					return isASTProvenLockFuncType(methodName, fn.Type)
				}
			}
		}
	}

	// 2. Check interface type specification
	ts := findTypeSpec(typeName, file)
	if ts != nil {
		if iface, ok := ts.Type.(*ast.InterfaceType); ok && iface.Methods != nil {
			for _, m := range iface.Methods.List {
				for _, name := range m.Names {
					if name.Name == methodName {
						if ft, ok := m.Type.(*ast.FuncType); ok {
							return isASTProvenLockFuncType(methodName, ft)
						}
					}
				}
			}
		}
	}

	return false
}

func isASTProvenLockFuncType(methodName string, ft *ast.FuncType) bool {
	if ft == nil || ft.Params == nil {
		return false
	}
	var paramTypes []ast.Expr
	for _, field := range ft.Params.List {
		if len(field.Names) == 0 {
			paramTypes = append(paramTypes, field.Type)
		} else {
			for range field.Names {
				paramTypes = append(paramTypes, field.Type)
			}
		}
	}
	if len(paramTypes) < 3 {
		return false
	}
	if !isASTContextType(paramTypes[0]) || !isASTStringType(paramTypes[2]) {
		return false
	}
	if methodName == "WithAdvisoryLock" || methodName == "ExecuteLockedTx" {
		for j := 2; j < len(paramTypes); j++ {
			if isASTFuncType(paramTypes[j]) {
				return true
			}
		}
		return false
	}
	return true
}

func isImportedArgusPackage(file *ast.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if isArgusPackagePath(path) {
			localName := "argus"
			if imp.Name != nil {
				localName = imp.Name.Name
			}
			if localName == name {
				return true
			}
		}
	}
	return false
}

func isIdentShadowed(file *ast.File, fn *ast.FuncDecl, pos token.Pos, name string) bool {
	if file != nil {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && (gen.Tok == token.VAR || gen.Tok == token.CONST) {
				for _, spec := range gen.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, p := range vs.Names {
							if p.Name == name {
								return true
							}
						}
					}
				}
			}
		}
	}
	if fn == nil {
		return false
	}
	if fn.Type != nil && fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, p := range field.Names {
				if p.Name == name {
					return true
				}
			}
		}
	}
	if fn.Body != nil {
		var shadowed bool
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if n == nil || n.Pos() >= pos {
				return false
			}
			switch s := n.(type) {
			case *ast.AssignStmt:
				if s.Tok == token.DEFINE {
					for _, lhs := range s.Lhs {
						if id, ok := lhs.(*ast.Ident); ok && id.Name == name {
							shadowed = true
							return false
						}
					}
				}
			case *ast.ValueSpec:
				for _, id := range s.Names {
					if id.Name == name {
						shadowed = true
						return false
					}
				}
			}
			return !shadowed
		})
		if shadowed {
			return true
		}
	}
	return false
}
