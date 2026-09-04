// Package a10_isolation_level delegates database type verification to the shared
// dbident package. This file provides thin wrappers maintaining package-internal
// function signatures for callers within a10_isolation_level.
package a10_isolation_level

import (
	"go/types"

	"github.com/will2469/argus/shared/dbident"
)

func isProvenDBPoolInterface(t types.Type) bool {
	return dbident.IsProvenDBPoolInterface(t)
}

func isProvenClosureTxType(t types.Type) bool {
	return dbident.IsProvenClosureTxType(t)
}

func isProvenDBTxType(t types.Type) bool {
	return dbident.IsProvenDBTxType(t)
}

func hasInvalidType(t types.Type) bool {
	return dbident.HasInvalidType(t)
}

