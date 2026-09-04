// Package a08_tx_io provides data-flow state tracking and lattice join operations
// for monitoring active database transaction objects across control-flow paths.
package a08_tx_io

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// TxState tracks active database transaction objects along an execution path.
type TxState struct {
	activeVars map[string]bool
	activeObjs map[types.Object]bool
	terminated bool
}

func newTxState() *TxState {
	return &TxState{
		activeVars: make(map[string]bool),
		activeObjs: make(map[types.Object]bool),
	}
}

func (s *TxState) clone() *TxState {
	c := newTxState()
	c.terminated = s.terminated
	for k, v := range s.activeVars {
		c.activeVars[k] = v
	}
	for k, v := range s.activeObjs {
		c.activeObjs[k] = v
	}
	return c
}

func (s *TxState) activate(pass *analysis.Pass, id *ast.Ident) {
	if id == nil {
		return
	}
	s.activeVars[id.Name] = true
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Defs[id]; obj != nil {
			s.activeObjs[obj] = true
		} else if obj := pass.TypesInfo.Uses[id]; obj != nil {
			s.activeObjs[obj] = true
		}
	}
}

func (s *TxState) deactivate(pass *analysis.Pass, id *ast.Ident) {
	if id == nil {
		return
	}
	delete(s.activeVars, id.Name)
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Uses[id]; obj != nil {
			delete(s.activeObjs, obj)
		}
	}
}

func (s *TxState) hasActive() bool {
	return len(s.activeVars) > 0 || len(s.activeObjs) > 0
}

// joinTxStates performs a fail-closed union lattice join across branch exit states.
func joinTxStates(states ...*TxState) *TxState {
	joined := newTxState()
	allTerminated := true

	for _, st := range states {
		if st == nil || st.terminated {
			continue
		}
		allTerminated = false
		for k := range st.activeVars {
			joined.activeVars[k] = true
		}
		for k := range st.activeObjs {
			joined.activeObjs[k] = true
		}
	}

	joined.terminated = allTerminated && len(states) > 0
	return joined
}
