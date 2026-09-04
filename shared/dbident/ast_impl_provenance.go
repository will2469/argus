// Package dbident provides AST-level implementation provenance verification
// for database interfaces and struct assignments in standalone runner mode.
package dbident

import (
	"go/ast"
)

// HasNonDBStructImplementation reports whether file defines a concrete struct
// that implements all methods of iface without containing any database driver fields.
func HasNonDBStructImplementation(file *ast.File, iface *ast.InterfaceType) bool {
	if file == nil || iface == nil || iface.Methods == nil || len(iface.Methods.List) == 0 {
		return false
	}

	// Map of structName -> map of implemented method names to FuncType
	structMethods := make(map[string]map[string]*ast.FuncType)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Type == nil {
			continue
		}
		recvTypeName := GetASTTypeName(fn.Recv.List[0].Type)
		if recvTypeName == "" {
			continue
		}
		if structMethods[recvTypeName] == nil {
			structMethods[recvTypeName] = make(map[string]*ast.FuncType)
		}
		structMethods[recvTypeName][fn.Name.Name] = fn.Type
	}

	for structName, methods := range structMethods {
		ts := FindTypeSpec(structName, file)
		if ts == nil {
			continue
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			continue
		}
		hasDBField := false
		if st.Fields != nil {
			for _, f := range st.Fields.List {
				if IsKnownDBDriverASTType(f.Type, file) {
					hasDBField = true
					break
				}
			}
		}
		if hasDBField {
			continue
		}

		// Check if this struct implements ALL methods of iface
		implementsAll := true
		for _, im := range iface.Methods.List {
			ift, ok := im.Type.(*ast.FuncType)
			if !ok {
				continue
			}
			for _, nm := range im.Names {
				smType, exists := methods[nm.Name]
				if !exists || !astFuncTypesMatch(ift, smType) {
					implementsAll = false
					break
				}
			}
			if !implementsAll {
				break
			}
		}

		if implementsAll {
			return true
		}
	}
	return false
}

func astFuncTypesMatch(f1, f2 *ast.FuncType) bool {
	if f1 == nil || f2 == nil {
		return false
	}
	p1Len := 0
	if f1.Params != nil {
		p1Len = len(f1.Params.List)
	}
	p2Len := 0
	if f2.Params != nil {
		p2Len = len(f2.Params.List)
	}
	if p1Len != p2Len {
		return false
	}
	r1Len := 0
	if f1.Results != nil {
		r1Len = len(f1.Results.List)
	}
	r2Len := 0
	if f2.Results != nil {
		r2Len = len(f2.Results.List)
	}
	return r1Len == r2Len
}

// IsAssignedFromNonDBConstructor reports whether expr was assigned in fn from
// a composite literal or constructor of a struct that lacks DB driver provenance.
func IsAssignedFromNonDBConstructor(expr ast.Expr, fn *ast.FuncDecl, file *ast.File) bool {
	id, ok := expr.(*ast.Ident)
	if !ok || fn == nil || fn.Body == nil || file == nil {
		return false
	}
	var isNonDB bool
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if isNonDB {
			return false
		}
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, lhs := range as.Lhs {
			if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name && i < len(as.Rhs) {
				rhs := as.Rhs[i]
				if u, ok := rhs.(*ast.UnaryExpr); ok {
					rhs = u.X
				}
				if comp, ok := rhs.(*ast.CompositeLit); ok {
					typeName := GetASTTypeName(comp.Type)
					if ts := FindTypeSpec(typeName, file); ts != nil {
						if st, ok := ts.Type.(*ast.StructType); ok {
							hasDBField := false
							if st.Fields != nil {
								for _, f := range st.Fields.List {
									if IsKnownDBDriverASTType(f.Type, file) {
										hasDBField = true
										break
									}
								}
							}
							if !hasDBField {
								isNonDB = true
								return false
							}
						}
					}
				}
			}
		}
		return true
	})
	return isNonDB
}
