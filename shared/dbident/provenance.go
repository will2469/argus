// Package dbident provides semantic and provenance-anchored database type
// verification for Go types and interfaces.
package dbident

import (
	"go/types"
)

// IsProvenDBQuerierType reports whether t is a proven database querier:
// strictly a concrete driver type or a struct wrapping a known DB driver.
// Custom interfaces cannot prove their implementation in isolation and
// must be verified in package context using IsProvenDBQuerierWithPkg.
func IsProvenDBQuerierType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = UnwrapPointer(t)

	return IsKnownDBDriverType(t)
}

func hasProvenDBQuerierMethods(iface *types.Interface) bool {
	if iface == nil {
		return false
	}

	hasExec := false
	hasQuery := false

	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		sig, ok := method.Type().(*types.Signature)
		if !ok {
			continue
		}

		switch method.Name() {
		case "Exec", "ExecContext":
			if isDBExecMethodSignature(sig) {
				hasExec = true
			}
		case "Query", "QueryContext", "QueryRow", "QueryRowContext":
			if isDBQueryMethodSignature(sig) {
				hasQuery = true
			}
		}
	}

	return hasExec && hasQuery
}

func isDBExecMethodSignature(sig *types.Signature) bool {
	if sig == nil || sig.Results() == nil || sig.Results().Len() != 2 {
		return false
	}
	if !IsKnownDBDriverType(sig.Results().At(0).Type()) {
		return false
	}
	if sig.Results().At(1).Type().String() != "error" {
		return false
	}
	return sigHasQueryString(sig)
}

func isDBQueryMethodSignature(sig *types.Signature) bool {
	if sig == nil || sig.Results() == nil {
		return false
	}
	if sig.Results().Len() == 2 {
		if !IsKnownDBDriverType(sig.Results().At(0).Type()) {
			return false
		}
		if sig.Results().At(1).Type().String() != "error" {
			return false
		}
		return sigHasQueryString(sig)
	}
	if sig.Results().Len() == 1 {
		if !IsKnownDBDriverType(sig.Results().At(0).Type()) {
			return false
		}
		return sigHasQueryString(sig)
	}
	return false
}

func sigHasQueryString(sig *types.Signature) bool {
	if sig == nil || sig.Params() == nil {
		return false
	}
	for i := 0; i < sig.Params().Len(); i++ {
		p := sig.Params().At(i).Type()
		if basic, ok := p.(*types.Basic); ok && basic.Kind() == types.String {
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
								if IsProvenDBTxType(cbParams.At(k).Type()) {
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
