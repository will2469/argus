// Package dbident provides implementation-level provenance verification
// for database interfaces, ensuring custom interfaces are backed by real
// database drivers and rejecting mock or non-DB implementations like Evil.
package dbident

import (
	"go/types"
)

// StructContainsKnownDBDriver reports whether t is a struct (or pointer to struct)
// that contains at least one field whose type is a known database driver type.
func StructContainsKnownDBDriver(t types.Type) bool {
	if t == nil {
		return false
	}
	t = UnwrapPointer(t)
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	return structFieldsContainDBDriver(st, 0)
}

func structFieldsContainDBDriver(st *types.Struct, depth int) bool {
	if st == nil || depth > 3 {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		ft := UnwrapPointer(st.Field(i).Type())
		if IsKnownDBDriverType(ft) {
			return true
		}
		if nested, ok := ft.Underlying().(*types.Struct); ok {
			if structFieldsContainDBDriver(nested, depth+1) {
				return true
			}
		}
	}
	return false
}

// HasNonDBImplementation reports whether any named concrete type in pkg implements
// iface but lacks database provenance (e.g., an Evil or mock struct with no DB fields).
func HasNonDBImplementation(iface *types.Interface, pkg *types.Package) bool {
	if iface == nil || pkg == nil {
		return false
	}
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		tn, ok := obj.(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		if _, isIface := named.Underlying().(*types.Interface); isIface {
			continue
		}
		implements := types.Implements(named, iface) || types.Implements(types.NewPointer(named), iface)
		if implements {
			if !IsKnownDBDriverType(named) && !StructContainsKnownDBDriver(named) {
				return true
			}
		}
	}
	return false
}

// HasDBProvenanceImplementation reports whether iface is implemented by a known DB
// driver type (e.g. *sql.DB) or by a struct within pkg that wraps a known DB driver.
func HasDBProvenanceImplementation(iface *types.Interface, pkg *types.Package) bool {
	if iface == nil {
		return false
	}
	if pkg != nil {
		scope := pkg.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			tn, ok := obj.(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			named, ok := tn.Type().(*types.Named)
			if !ok {
				continue
			}
			if StructContainsKnownDBDriver(named) {
				if types.Implements(named, iface) || types.Implements(types.NewPointer(named), iface) {
					return true
				}
			}
		}
		for _, imp := range pkg.Imports() {
			if !IsKnownDBPackagePath(imp.Path()) {
				continue
			}
			for typeName := range knownDBDriverTypeNames {
				if obj := imp.Scope().Lookup(typeName); obj != nil {
					t := obj.Type()
					if types.Implements(t, iface) || types.Implements(types.NewPointer(t), iface) {
						return true
					}
				}
			}
		}
	}
	return false
}

// IsProvenDBQuerierWithPkg reports whether t is a proven database querier in the
// context of pkg: either a concrete driver type, or a custom interface that
// satisfies the full querier contract (Exec AND Query returning driver types)
// and is provably free from non-DB mock implementations (like Evil).
func IsProvenDBQuerierWithPkg(t types.Type, pkg *types.Package) bool {
	if t == nil {
		return false
	}
	t = UnwrapPointer(t)
	if IsKnownDBDriverType(t) {
		return true
	}
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	if !hasProvenDBQuerierMethods(iface) {
		return false
	}
	if pkg != nil && HasNonDBImplementation(iface, pkg) {
		return false
	}
	return true
}
