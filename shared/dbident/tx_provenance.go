// Package dbident provides semantic and provenance-anchored transaction type
// and interface contract verification for Go database transactions.
package dbident

import (
	"go/types"
)

// IsProvenDBTxType reports whether t is a proven database transaction type:
// strictly a concrete Tx from a known database package (sql.Tx, pgx.Tx, sqlx.Tx)
// that manages physical connection checkout and connection-pool locking.
// User-defined interfaces (FakeTx) are NEVER accepted as database transactions.
func IsProvenDBTxType(t types.Type) bool {
	return IsKnownDBTxType(t)
}

// IsProvenClosureTxType reports whether t is a suitable transaction type
// for closure-based transaction APIs (BeginFunc callbacks). Requires
// a proven database transaction type.
func IsProvenClosureTxType(t types.Type) bool {
	return IsKnownDBTxType(t)
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

