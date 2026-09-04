// Package a03_context provides flow-sensitive and scope-aware tracking of context values.
package a03_context

import (
	"go/types"
)

type contextKind int

const (
	ctxUnknown contextKind = iota
	ctxBounded
	ctxRaw
)

type flowState struct {
	objects map[types.Object]contextKind
	scopes  []map[string]contextKind
}

func newFlowState() *flowState {
	return &flowState{
		objects: make(map[types.Object]contextKind),
		scopes:  []map[string]contextKind{make(map[string]contextKind)},
	}
}

func (s *flowState) clone() *flowState {
	cloned := &flowState{
		objects: make(map[types.Object]contextKind, len(s.objects)),
		scopes:  make([]map[string]contextKind, len(s.scopes)),
	}
	for k, v := range s.objects {
		cloned.objects[k] = v
	}
	for i, sc := range s.scopes {
		newSc := make(map[string]contextKind, len(sc))
		for k, v := range sc {
			newSc[k] = v
		}
		cloned.scopes[i] = newSc
	}
	return cloned
}

func (s *flowState) pushScope() {
	s.scopes = append(s.scopes, make(map[string]contextKind))
}

func (s *flowState) popScope() {
	if len(s.scopes) > 1 {
		s.scopes = s.scopes[:len(s.scopes)-1]
	}
}

func (s *flowState) define(name string, obj types.Object, kind contextKind) {
	if obj != nil {
		s.objects[obj] = kind
	}
	if len(s.scopes) > 0 {
		s.scopes[len(s.scopes)-1][name] = kind
	}
}

func (s *flowState) update(name string, obj types.Object, kind contextKind) {
	if obj != nil {
		s.objects[obj] = kind
	}
	for i := len(s.scopes) - 1; i >= 0; i-- {
		if _, exists := s.scopes[i][name]; exists {
			s.scopes[i][name] = kind
			return
		}
	}
	s.define(name, obj, kind)
}

func (s *flowState) lookup(name string, obj types.Object) contextKind {
	if obj != nil {
		if k, exists := s.objects[obj]; exists {
			return k
		}
	}
	for i := len(s.scopes) - 1; i >= 0; i-- {
		if k, exists := s.scopes[i][name]; exists {
			return k
		}
	}
	return ctxUnknown
}

func joinStates(s1, s2 *flowState) *flowState {
	merged := newFlowState()
	for obj, k1 := range s1.objects {
		k2 := s2.objects[obj]
		merged.objects[obj] = joinKind(k1, k2)
	}
	for obj, k2 := range s2.objects {
		if _, seen := merged.objects[obj]; !seen {
			merged.objects[obj] = joinKind(s1.objects[obj], k2)
		}
	}

	maxScopes := len(s1.scopes)
	if len(s2.scopes) > maxScopes {
		maxScopes = len(s2.scopes)
	}
	merged.scopes = make([]map[string]contextKind, maxScopes)
	for i := 0; i < maxScopes; i++ {
		merged.scopes[i] = make(map[string]contextKind)
		var sc1, sc2 map[string]contextKind
		if i < len(s1.scopes) {
			sc1 = s1.scopes[i]
		}
		if i < len(s2.scopes) {
			sc2 = s2.scopes[i]
		}
		for k, v1 := range sc1 {
			v2 := sc2[k]
			merged.scopes[i][k] = joinKind(v1, v2)
		}
		for k, v2 := range sc2 {
			if _, seen := merged.scopes[i][k]; !seen {
				merged.scopes[i][k] = joinKind(sc1[k], v2)
			}
		}
	}
	return merged
}

func joinKind(k1, k2 contextKind) contextKind {
	if k1 == ctxRaw || k2 == ctxRaw {
		return ctxRaw // Any reachable path with raw context is a vulnerability
	}
	if k1 == ctxBounded && k2 == ctxBounded {
		return ctxBounded
	}
	if k1 == ctxBounded || k2 == ctxBounded {
		return ctxBounded
	}
	return ctxUnknown
}
