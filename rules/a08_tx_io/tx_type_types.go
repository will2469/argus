// Package a08_tx_io provides type checking and proof-level verification for
// database connection pools, transactions, and closure signatures.
package a08_tx_io

import (
	"go/types"
)

func isProvenDBPoolInterface(t types.Type) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)

	// 1. Direct match for known DB pool / connection types
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		pkg := named.Obj().Pkg()
		if pkg != nil && isKnownDBPackagePath(pkg.Path()) {
			switch named.Obj().Name() {
			case "DB", "Pool", "Conn":
				return true
			}
		}
	}

	// 2. Interface contract verification
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
					if isProvenDBTxType(results.At(j).Type()) {
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
								if isProvenClosureTxType(cbParams.At(k).Type()) {
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

func isProvenClosureTxType(t types.Type) bool {
	if isProvenDBTxType(t) {
		return true
	}
	t = unwrapPointer(t)
	if iface, ok := t.Underlying().(*types.Interface); ok {
		for i := 0; i < iface.NumMethods(); i++ {
			switch iface.Method(i).Name() {
			case "Exec", "ExecContext", "Query", "QueryRow", "QueryContext", "QueryRowContext", "Commit", "Rollback":
				return true
			}
		}
	}
	return false
}

func isProvenDBTxType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = unwrapPointer(t)

	// 1. Direct driver types: *sql.Tx, pgx.Tx
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		pkg := named.Obj().Pkg()
		if pkg != nil && isKnownDBPackagePath(pkg.Path()) {
			switch named.Obj().Name() {
			case "Tx":
				return true
			}
		}
	}

	// 2. Interface contract verification
	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}

	return hasProvenDBTxMethods(iface)
}

func hasProvenDBTxMethods(iface *types.Interface) bool {
	if iface == nil {
		return false
	}

	hasCommit := false
	hasRollback := false
	hasExecOrQuery := false

	for i := 0; i < iface.NumMethods(); i++ {
		switch iface.Method(i).Name() {
		case "Commit":
			hasCommit = true
		case "Rollback":
			hasRollback = true
		case "Exec", "ExecContext", "Query", "QueryRow", "QueryContext", "QueryRowContext", "SendBatch":
			hasExecOrQuery = true
		}
	}

	return hasCommit && hasRollback && hasExecOrQuery
}

func isKnownDBPackagePath(path string) bool {
	switch path {
	case "database/sql", "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v5/pgxpool",
		"github.com/jackc/pgx/v5/pgconn", "github.com/jackc/pgx/v4", "github.com/jackc/pgx/v4/pgxpool",
		"github.com/jmoiron/sqlx", "github.com/lib/pq":
		return true
	}
	return false
}

func hasInvalidType(t types.Type) bool {
	if t == nil {
		return true
	}
	switch x := t.(type) {
	case *types.Basic:
		return x.Kind() == types.Invalid
	case *types.Pointer:
		return hasInvalidType(x.Elem())
	case *types.Named:
		return hasInvalidType(x.Underlying())
	}
	return false
}
