// Package a06_runtime_ddl maintains flow-sensitive DDL tracking state using a fail-closed lattice.
package a06_runtime_ddl

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// varKey uniquely identifies a variable either via compiler types.Object
// or via lexical declaration position and name in standalone mode.
type varKey struct {
	obj     types.Object
	declPos token.Pos
	name    string
}

type ddlKind int

const (
	ddlKindUnknown ddlKind = iota
	ddlKindClean
	ddlKindDefinite
	ddlKindMaybe
)

type ddlValue struct {
	kind ddlKind
	op   string
}

func (v ddlValue) isDDL() bool {
	return v.kind == ddlKindDefinite || v.kind == ddlKindMaybe
}

func (v ddlValue) getOp() string {
	if v.isDDL() {
		return v.op
	}
	return ""
}

func joinValues(a, b ddlValue) ddlValue {
	if a == b {
		return a
	}
	if a.kind == ddlKindUnknown {
		return b
	}
	if b.kind == ddlKindUnknown {
		return a
	}

	if a.isDDL() || b.isDDL() {
		op := a.op
		if op == "" {
			op = b.op
		}
		if a.kind == ddlKindDefinite && b.kind == ddlKindDefinite && a.op == b.op {
			return a
		}
		return ddlValue{kind: ddlKindMaybe, op: op}
	}

	if a.kind == ddlKindClean && b.kind == ddlKindClean {
		return ddlValue{kind: ddlKindClean}
	}

	return ddlValue{kind: ddlKindUnknown}
}

type ddlState struct {
	vars map[varKey]ddlValue
}

func newDDLState() *ddlState {
	return &ddlState{
		vars: make(map[varKey]ddlValue),
	}
}

func (s *ddlState) clone() *ddlState {
	c := newDDLState()
	for k, v := range s.vars {
		c.vars[k] = v
	}
	return c
}

func (s *ddlState) join(other *ddlState) *ddlState {
	if other == nil {
		return s.clone()
	}
	c := newDDLState()

	allKeys := make(map[varKey]struct{})
	for k := range s.vars {
		allKeys[k] = struct{}{}
	}
	for k := range other.vars {
		allKeys[k] = struct{}{}
	}

	for k := range allKeys {
		vA := s.vars[k]
		vB := other.vars[k]
		joined := joinValues(vA, vB)
		if joined.kind != ddlKindUnknown {
			c.vars[k] = joined
		}
	}

	return c
}

func (s *ddlState) set(k varKey, val ddlValue) {
	s.vars[k] = val
}

func (s *ddlState) get(k varKey) (ddlValue, bool) {
	v, ok := s.vars[k]
	return v, ok
}

func makeVarKey(pass *analysis.Pass, file *ast.File, fn *ast.FuncDecl, id *ast.Ident) varKey {
	if id == nil {
		return varKey{}
	}
	if pass != nil && pass.TypesInfo != nil {
		obj := pass.TypesInfo.Uses[id]
		if obj == nil {
			obj = pass.TypesInfo.Defs[id]
		}
		if obj != nil {
			return varKey{obj: obj, name: id.Name}
		}
	}
	declPos := findDeclPos(id, fn, file)
	return varKey{declPos: declPos, name: id.Name}
}

func makeDefVarKey(pass *analysis.Pass, id *ast.Ident) varKey {
	if id == nil {
		return varKey{}
	}
	if pass != nil && pass.TypesInfo != nil {
		obj := pass.TypesInfo.Defs[id]
		if obj == nil {
			obj = pass.TypesInfo.Uses[id]
		}
		if obj != nil {
			return varKey{obj: obj, name: id.Name}
		}
	}
	return varKey{declPos: id.Pos(), name: id.Name}
}

func isStringBuilderType(t types.Type) bool {
	if t == nil {
		return false
	}
	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		if pkg := named.Obj().Pkg(); pkg != nil {
			p := pkg.Path()
			n := named.Obj().Name()
			if (p == "strings" && n == "Builder") || (p == "bytes" && n == "Buffer") {
				return true
			}
		}
		lower := strings.ToLower(named.Obj().Name())
		return strings.Contains(lower, "builder") || strings.Contains(lower, "buffer")
	}
	return false
}
