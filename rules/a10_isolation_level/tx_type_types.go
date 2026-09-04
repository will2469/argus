// Package a10_isolation_level provides semantic type checking and proof-level verification
// for database connection pools, transactions, and closure signatures.
package a10_isolation_level

import (
	"go/types"
	"strings"
)

func isProvenDBPoolInterface(t types.Type) bool {
	t = unwrapPointer(t)
	if t == nil {
		return false
	}

	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		typeName := named.Obj().Name()
		if isKnownNonDBTypeName(typeName) {
			return false
		}
		if pkg := named.Obj().Pkg(); pkg != nil && isKnownDBPackagePath(pkg.Path()) {
			switch typeName {
			case "DB", "Pool", "Conn":
				return true
			}
		}
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
	if isKnownNonDBTypeName(getTypeName(t)) {
		return false
	}
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
	t = unwrapPointer(t)
	if t == nil {
		return false
	}

	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		typeName := named.Obj().Name()
		if isKnownNonDBTypeName(typeName) {
			return false
		}
		if obj := named.Obj(); obj.Pkg() != nil && isKnownDBPackagePath(obj.Pkg().Path()) {
			if obj.Name() == "Tx" {
				return true
			}
		}
	}

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
		case "Exec", "ExecContext", "Query", "QueryRow", "QueryContext", "QueryRowContext":
			if isDBExecOrQuerySignature(sig) {
				hasExecOrQuery = true
			}
		case "SendBatch":
			hasExecOrQuery = true
		}
	}

	return hasCommit && hasRollback && hasExecOrQuery
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
	case *types.Struct:
		for i := 0; i < x.NumFields(); i++ {
			if hasInvalidType(x.Field(i).Type()) {
				return true
			}
		}
	case *types.Interface:
		for i := 0; i < x.NumMethods(); i++ {
			if hasInvalidType(x.Method(i).Type()) {
				return true
			}
		}
	case *types.Signature:
		if results := x.Results(); results != nil {
			for i := 0; i < results.Len(); i++ {
				if hasInvalidType(results.At(i).Type()) {
					return true
				}
			}
		}
		if params := x.Params(); params != nil {
			for i := 0; i < params.Len(); i++ {
				if hasInvalidType(params.At(i).Type()) {
					return true
				}
			}
		}
	}
	return false
}
