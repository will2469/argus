// Package dbident provides semantic and provenance-anchored transaction type
// and interface contract verification for Go database transactions.
package dbident

import (
	"go/types"
)

// IsProvenDBTxType reports whether t is a proven database transaction type:
// either a concrete Tx from a known package, or an interface with
// Commit+Rollback+ExecOrQuery where at least one method's signature
// references a concrete driver type (provenance anchor).
func IsProvenDBTxType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = UnwrapPointer(t)

	if IsKnownDBTxType(t) {
		return true
	}

	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	return hasProvenDBTxMethods(iface)
}

// IsProvenClosureTxType reports whether t is a suitable transaction type
// for closure-based transaction APIs (BeginFunc callbacks). Requires
// a proven database transaction type.
func IsProvenClosureTxType(t types.Type) bool {
	return IsProvenDBTxType(t)
}

// IsExactContextType reports whether t is the exact context.Context interface
// from the standard library "context" package.
func IsExactContextType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = UnwrapPointer(t)
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil {
		return false
	}
	pkg := named.Obj().Pkg()
	return pkg != nil && pkg.Path() == "context" && named.Obj().Name() == "Context"
}

// hasProvenDBTxMethods checks an interface for transaction semantics.
// An interface is accepted as a DB transaction IF AND ONLY IF it has
// verified driver provenance AND the standard transaction lifecycle methods
// (Commit + Rollback + Exec/Query).
func hasProvenDBTxMethods(iface *types.Interface) bool {
	if iface == nil {
		return false
	}

	hasCommit := false
	hasRollback := false
	hasExecOrQuery := false
	hasProvenance := false

	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		sig, ok := method.Type().(*types.Signature)
		if !ok {
			continue
		}

		switch method.Name() {
		case "Commit":
			if isTxLifecycleSignature(sig) {
				hasCommit = true
			}
		case "Rollback":
			if isTxLifecycleSignature(sig) {
				hasRollback = true
			}
		case "Exec", "ExecContext":
			if isDBExecOrQuerySignature(sig) {
				hasExecOrQuery = true
			}
		case "Query", "QueryRow", "QueryContext", "QueryRowContext", "SendBatch":
			if isDBExecOrQuerySignature(sig) {
				hasExecOrQuery = true
			}
		}

		if hasDriverTypeInSignature(method) {
			hasProvenance = true
		}
	}

	return hasProvenance && hasCommit && hasRollback && hasExecOrQuery
}

func isTxLifecycleSignature(sig *types.Signature) bool {
	if sig == nil || sig.Results() == nil || sig.Results().Len() != 1 {
		return false
	}
	res := sig.Results().At(0).Type()
	if res.String() != "error" {
		return false
	}
	if sig.Params() != nil {
		if sig.Params().Len() == 0 {
			return true
		}
		if sig.Params().Len() == 1 {
			return IsExactContextType(sig.Params().At(0).Type())
		}
		return false
	}
	return true
}

func isDBExecOrQuerySignature(sig *types.Signature) bool {
	if sig == nil || sig.Params() == nil {
		return false
	}
	for i := 0; i < sig.Params().Len(); i++ {
		paramType := sig.Params().At(i).Type()
		if basic, ok := paramType.(*types.Basic); ok && basic.Kind() == types.String {
			return true
		}
	}
	return false
}
