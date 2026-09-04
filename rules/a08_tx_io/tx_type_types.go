// Package a08_tx_io delegates database type verification to the shared
// dbident package. This file provides thin wrappers maintaining the
// package-internal function signatures for callers within a08_tx_io.
package a08_tx_io

import (
	"go/types"

	"github.com/will2469/argus/shared/dbident"
)

func isProvenDBPoolInterface(t types.Type) bool {
	return dbident.IsProvenDBPoolInterface(t)
}

func isProvenDBTxType(t types.Type) bool {
	return dbident.IsProvenDBTxType(t)
}

func hasInvalidType(t types.Type) bool {
	return dbident.HasInvalidType(t)
}

func unwrapPointer(t types.Type) types.Type {
	return dbident.UnwrapPointer(t)
}
