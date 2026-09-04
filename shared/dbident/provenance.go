// Package dbident provides semantic and provenance-anchored database type
// verification for Go types and interfaces.
package dbident

import (
	"go/types"
)

// IsProvenDBQuerierType reports whether t is a proven database querier:
// either a concrete driver type, or an interface whose method signatures
// reference concrete driver types (provenance-anchored).
func IsProvenDBQuerierType(t types.Type) bool {
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

	for i := 0; i < iface.NumMethods(); i++ {
		if isDBMethodWithDriverProvenance(iface.Method(i)) {
			return true
		}
	}
	return false
}

// IsProvenDBPoolInterface reports whether t is a proven database pool type:
// either a concrete pool type from a known package, or an interface with
// Begin/BeginTx returning a proven Tx type, or BeginFunc/WithTx/ExecuteTx
// accepting a callback with a proven Tx parameter.
func IsProvenDBPoolInterface(t types.Type) bool {
	if t == nil {
		return false
	}
	t = UnwrapPointer(t)

	if IsKnownDBPoolType(t) {
		return true
	}

	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	return hasProvenDBPoolMethods(iface)
}

func hasProvenDBPoolMethods(iface *types.Interface) bool {
	if iface == nil {
		return false
	}

	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		sig, ok := method.Type().(*types.Signature)
		if !ok {
			continue
		}

		switch method.Name() {
		case "Begin", "BeginTx":
			if results := sig.Results(); results != nil && results.Len() > 0 {
				for j := 0; j < results.Len(); j++ {
					if IsProvenDBTxType(results.At(j).Type()) {
						return true
					}
				}
			}
		case "BeginFunc", "WithTx", "ExecuteTx":
			if params := sig.Params(); params != nil {
				for j := 0; j < params.Len(); j++ {
					if cbSig, ok := params.At(j).Type().Underlying().(*types.Signature); ok {
						if cbParams := cbSig.Params(); cbParams != nil && cbParams.Len() > 0 {
							for k := 0; k < cbParams.Len(); k++ {
								if IsProvenClosureTxType(cbParams.At(k).Type()) {
									return true
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

// IsDBMethodWithDriverSignature checks if a method has a DB-related name
// and its signature references concrete driver types.
func IsDBMethodWithDriverSignature(fn *types.Func) bool {
	return isDBMethodWithDriverProvenance(fn)
}

func isDBMethodWithDriverProvenance(fn *types.Func) bool {
	if fn == nil {
		return false
	}
	switch fn.Name() {
	case "Query", "QueryRow", "Exec", "ExecContext",
		"Begin", "BeginTx", "SendBatch":
	default:
		return false
	}
	return hasDriverTypeInSignature(fn)
}

func hasDriverTypeInSignature(fn *types.Func) bool {
	if fn == nil {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}

	if results := sig.Results(); results != nil {
		for i := 0; i < results.Len(); i++ {
			if IsKnownDBDriverType(results.At(i).Type()) {
				return true
			}
		}
	}

	if params := sig.Params(); params != nil {
		for i := 0; i < params.Len(); i++ {
			if IsKnownDBDriverType(params.At(i).Type()) {
				return true
			}
		}
	}

	return false
}
