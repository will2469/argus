package a01_sql_concat

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

type taintState struct {
	objects map[types.Object]struct{}
	names   map[string]struct{}
}

func newTaintState() *taintState {
	return &taintState{
		objects: make(map[types.Object]struct{}),
		names:   make(map[string]struct{}),
	}
}

func (s *taintState) clone() *taintState {
	c := newTaintState()
	for k := range s.objects {
		c.objects[k] = struct{}{}
	}
	for k := range s.names {
		c.names[k] = struct{}{}
	}
	return c
}

func (s *taintState) join(other *taintState) *taintState {
	if other == nil {
		return s.clone()
	}
	c := s.clone()
	for k := range other.objects {
		c.objects[k] = struct{}{}
	}
	for k := range other.names {
		c.names[k] = struct{}{}
	}
	return c
}

func (s *taintState) markTainted(ident *ast.Ident, pass *analysis.Pass) {
	if ident == nil {
		return
	}
	s.names[ident.Name] = struct{}{}
	if pass != nil && pass.TypesInfo != nil {
		obj := pass.TypesInfo.Defs[ident]
		if obj == nil {
			obj = pass.TypesInfo.Uses[ident]
		}
		if obj != nil {
			s.objects[obj] = struct{}{}
		}
	}
}

func (s *taintState) markClean(ident *ast.Ident, pass *analysis.Pass) {
	if ident == nil {
		return
	}
	delete(s.names, ident.Name)
	if pass != nil && pass.TypesInfo != nil {
		obj := pass.TypesInfo.Defs[ident]
		if obj == nil {
			obj = pass.TypesInfo.Uses[ident]
		}
		if obj != nil {
			delete(s.objects, obj)
		}
	}
}

func (s *taintState) isIdentTainted(ident *ast.Ident, pass *analysis.Pass) bool {
	if ident == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Uses[ident]; obj != nil {
			if _, ok := s.objects[obj]; ok {
				return true
			}
		}
	}
	_, ok := s.names[ident.Name]
	return ok
}
