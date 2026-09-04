// Package a06_runtime_ddl maintains flow-sensitive DDL tracking state.
package a06_runtime_ddl

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

type ddlState struct {
	vars map[string]string
	objs map[types.Object]string
}

func newDDLState() *ddlState {
	return &ddlState{
		vars: make(map[string]string),
		objs: make(map[types.Object]string),
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
	c := s.clone()
	for k, v := range other.vars {
		c.vars[k] = v
	}
	for k, v := range other.objs {
		c.objs[k] = v
	}
	return c
}

func (s *ddlState) markDDL(id *ast.Ident, op string, pass *analysis.Pass) {
	if id == nil || op == "" {
		return
	}
	s.vars[id.Name] = op
	if pass != nil && pass.TypesInfo != nil {
		obj := pass.TypesInfo.Defs[id]
		if obj == nil {
			obj = pass.TypesInfo.Uses[id]
		}
		if obj != nil {
			s.objs[obj] = op
		}
	}
}

func (s *ddlState) markClean(id *ast.Ident, pass *analysis.Pass) {
	if id == nil {
		return
	}
	delete(s.vars, id.Name)
	if pass != nil && pass.TypesInfo != nil {
		obj := pass.TypesInfo.Defs[id]
		if obj == nil {
			obj = pass.TypesInfo.Uses[id]
		}
		if obj != nil {
			delete(s.objs, obj)
		}
	}
}

func (s *ddlState) getOp(id *ast.Ident, pass *analysis.Pass) string {
	if id == nil {
		return ""
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Uses[id]; obj != nil {
			if op, ok := s.objs[obj]; ok {
				return op
			}
		}
	}
	return s.vars[id.Name]
}
