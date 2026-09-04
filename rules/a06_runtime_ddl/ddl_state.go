// Package a06_runtime_ddl maintains flow-sensitive DDL tracking state using a fail-closed lattice.
package a06_runtime_ddl

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

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
	op := a.op
	if op == "" {
		op = b.op
	}

	if a.isDDL() || b.isDDL() {
		if a.kind == ddlKindDefinite && b.kind == ddlKindDefinite && a.op == b.op {
			return a
		}
		return ddlValue{kind: ddlKindMaybe, op: op}
	}

	if a.kind == ddlKindClean || b.kind == ddlKindClean {
		return ddlValue{kind: ddlKindClean}
	}

	return ddlValue{kind: ddlKindUnknown}
}

type ddlState struct {
	vars map[string]ddlValue
	objs map[types.Object]ddlValue
}

func newDDLState() *ddlState {
	return &ddlState{
		vars: make(map[string]ddlValue),
		objs: make(map[types.Object]ddlValue),
	}
}

func (s *ddlState) clone() *ddlState {
	c := newDDLState()
	for k, v := range s.vars {
		c.vars[k] = v
	}
	for k, v := range s.objs {
		c.objs[k] = v
	}
	return c
}

func (s *ddlState) join(other *ddlState) *ddlState {
	if other == nil {
		return s.clone()
	}
	c := newDDLState()

	varKeys := make(map[string]struct{})
	for k := range s.vars {
		varKeys[k] = struct{}{}
	}
	for k := range other.vars {
		varKeys[k] = struct{}{}
	}
	for k := range varKeys {
		vA := s.vars[k]
		vB := other.vars[k]
		joined := joinValues(vA, vB)
		if joined.kind != ddlKindUnknown {
			c.vars[k] = joined
		}
	}

	objKeys := make(map[types.Object]struct{})
	for k := range s.objs {
		objKeys[k] = struct{}{}
	}
	for k := range other.objs {
		objKeys[k] = struct{}{}
	}
	for k := range objKeys {
		vA := s.objs[k]
		vB := other.objs[k]
		joined := joinValues(vA, vB)
		if joined.kind != ddlKindUnknown {
			c.objs[k] = joined
		}
	}

	return c
}

func (s *ddlState) markDDL(id *ast.Ident, op string, pass *analysis.Pass) {
	if id == nil || op == "" {
		return
	}
	val := ddlValue{kind: ddlKindDefinite, op: op}
	s.vars[id.Name] = val
	if pass != nil && pass.TypesInfo != nil {
		obj := pass.TypesInfo.Defs[id]
		if obj == nil {
			obj = pass.TypesInfo.Uses[id]
		}
		if obj != nil {
			s.objs[obj] = val
		}
	}
}

func (s *ddlState) markClean(id *ast.Ident, pass *analysis.Pass) {
	if id == nil {
		return
	}
	val := ddlValue{kind: ddlKindClean}
	s.vars[id.Name] = val
	if pass != nil && pass.TypesInfo != nil {
		obj := pass.TypesInfo.Defs[id]
		if obj == nil {
			obj = pass.TypesInfo.Uses[id]
		}
		if obj != nil {
			s.objs[obj] = val
		}
	}
}

func (s *ddlState) getOp(id *ast.Ident, pass *analysis.Pass) string {
	if id == nil {
		return ""
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Uses[id]; obj != nil {
			if v, ok := s.objs[obj]; ok {
				return v.getOp()
			}
		}
	}
	return s.vars[id.Name].getOp()
}

func isStringBuilderExpr(pass *analysis.Pass, expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		tv := pass.TypesInfo.TypeOf(expr)
		if tv != nil && tv != types.Typ[types.Invalid] {
			return isStringBuilderType(tv)
		}
	}
	if id, ok := expr.(*ast.Ident); ok {
		lower := strings.ToLower(id.Name)
		return lower == "b" || lower == "sb" || lower == "buf" ||
			strings.Contains(lower, "builder") || strings.Contains(lower, "buffer") ||
			strings.Contains(lower, "query") || strings.Contains(lower, "sql")
	}
	return false
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
