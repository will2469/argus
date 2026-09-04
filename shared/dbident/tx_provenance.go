// Package dbident provides semantic and provenance-anchored transaction type
// and interface contract verification for Go database transactions.
package dbident

import (
	"go/types"
	"strings"
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
// either a proven direct Tx type or a provenance-anchored Tx interface.
func IsProvenClosureTxType(t types.Type) bool {
	if IsProvenDBTxType(t) {
		return true
	}
	t = UnwrapPointer(t)
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for i := 0; i < iface.NumMethods(); i++ {
		if hasDriverTypeInSignature(iface.Method(i)) {
			return true
		}
	}
	return false
}

// hasProvenDBTxMethods checks an interface for transaction semantics.
// An interface is accepted as a DB transaction if:
// 1. (Path 1) It has driver provenance AND standard Tx shape.
// 2. (Path 2) It implements the complete database transaction contract:
//    Commit + Rollback + Exec + Query, where Exec/Query take context.Context
//    and a query string.
func hasProvenDBTxMethods(iface *types.Interface) bool {
	if iface == nil {
		return false
	}

	hasCommit := false
	hasRollback := false
	hasExec := false
	hasQuery := false
	hasExecOrQuery := false
	hasProvenance := false
	hasContext := false

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
				hasExec = true
				hasExecOrQuery = true
			}
			if sigHasContext(sig) {
				hasContext = true
			}
		case "Query", "QueryRow", "QueryContext", "QueryRowContext":
			if isDBExecOrQuerySignature(sig) {
				hasQuery = true
				hasExecOrQuery = true
			}
			if sigHasContext(sig) {
				hasContext = true
			}
		case "SendBatch":
			hasExecOrQuery = true
		}

		if hasDriverTypeInSignature(method) {
			hasProvenance = true
		}
	}

	if hasProvenance && hasCommit && hasRollback && hasExecOrQuery {
		return true
	}
	if hasCommit && hasRollback && hasExec && hasQuery && hasContext {
		return true
	}

	return false
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
			paramType := sig.Params().At(0).Type().String()
			return strings.Contains(paramType, "context.Context")
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

func sigHasContext(sig *types.Signature) bool {
	if sig == nil || sig.Params() == nil || sig.Params().Len() == 0 {
		return false
	}
	first := sig.Params().At(0).Type().String()
	return strings.Contains(first, "context.Context")
}
