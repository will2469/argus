// Package a07_error_leak provides flow-sensitive abstract state tracking and canonical object identity.
package a07_error_leak

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// errorKind defines the abstract domain for an error value.
type errorKind int

const (
	// errorKindClean represents a provably safe/non-database origin (errors.New, non-DB service calls, etc.).
	errorKindClean errorKind = iota
	// errorKindGenericParam represents an unclassified function error parameter (e.g. err error).
	errorKindGenericParam
	// errorKindDB represents a proven database origin (db.Query, db.Exec, pgconn.PgError, etc.).
	errorKindDB
)

// errorValue represents an abstract error state at a specific program point.
type errorValue struct {
	kind   errorKind
	source string
}

// varKey uniquely identifies a variable by compiler types.Object (pass mode)
// or lexical declaration position + name (standalone mode).
type varKey struct {
	obj     types.Object
	declPos token.Pos
	name    string
}

// errorState tracks reaching error values mapped by varKey.
type errorState struct {
	vars map[varKey]errorValue
}

func newErrorState() *errorState {
	return &errorState{vars: make(map[varKey]errorValue)}
}

func (s *errorState) clone() *errorState {
	if s == nil {
		return newErrorState()
	}
	cp := newErrorState()
	for k, v := range s.vars {
		cp.vars[k] = v
	}
	return cp
}

func (s *errorState) set(k varKey, val errorValue) {
	if s == nil {
		return
	}
	s.vars[k] = val
}

func (s *errorState) get(k varKey) errorValue {
	if s == nil {
		return errorValue{kind: errorKindClean}
	}
	return s.vars[k]
}

// join computes the abstract lattice least upper bound:
// CLEAN ⊔ CLEAN = CLEAN
// CLEAN ⊔ GENERIC_PARAM = GENERIC_PARAM
// CLEAN ⊔ DB = DB
// GENERIC_PARAM ⊔ DB = DB
func (s *errorState) join(other *errorState) *errorState {
	if s == nil && other == nil {
		return newErrorState()
	}
	if s == nil {
		return other.clone()
	}
	if other == nil {
		return s.clone()
	}

	res := newErrorState()
	allKeys := make(map[varKey]bool)
	for k := range s.vars {
		allKeys[k] = true
	}
	for k := range other.vars {
		allKeys[k] = true
	}

	for k := range allKeys {
		v1, ok1 := s.vars[k]
		v2, ok2 := other.vars[k]
		if !ok1 {
			res.vars[k] = v2
			continue
		}
		if !ok2 {
			res.vars[k] = v1
			continue
		}
		res.vars[k] = joinValues(v1, v2)
	}

	return res
}

func joinValues(v1, v2 errorValue) errorValue {
	if v1.kind == errorKindDB || v2.kind == errorKindDB {
		src := v1.source
		if src == "" {
			src = v2.source
		}
		return errorValue{kind: errorKindDB, source: src}
	}
	if v1.kind == errorKindGenericParam || v2.kind == errorKindGenericParam {
		return errorValue{kind: errorKindGenericParam, source: "param"}
	}
	return errorValue{kind: errorKindClean}
}

// makeVarKey resolves canonical variable key for a use or assignment site.
func makeVarKey(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, id *ast.Ident) varKey {
	if id == nil {
		return varKey{}
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Uses[id]; obj != nil {
			return varKey{obj: obj, declPos: obj.Pos(), name: id.Name}
		}
		if obj := pass.TypesInfo.Defs[id]; obj != nil {
			return varKey{obj: obj, declPos: obj.Pos(), name: id.Name}
		}
	}
	declPos := findDeclPos(file, fn, id)
	return varKey{declPos: declPos, name: id.Name}
}

// makeDefVarKey resolves canonical variable key for a definition site (:= or var).
func makeDefVarKey(pass *analysis.Pass, id *ast.Ident) varKey {
	if id == nil {
		return varKey{}
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Defs[id]; obj != nil {
			return varKey{obj: obj, declPos: obj.Pos(), name: id.Name}
		}
	}
	return varKey{declPos: id.Pos(), name: id.Name}
}

