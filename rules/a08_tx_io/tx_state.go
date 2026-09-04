// Package a08_tx_io provides data-flow state tracking and lattice join operations
// for monitoring active database transaction objects across control-flow paths.
package a08_tx_io

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// varKey uniquely identifies a transaction variable by compiler types.Object (pass mode)
// or lexical declaration position + name (standalone mode).
type varKey struct {
	obj     types.Object
	declPos token.Pos
	name    string
}

func makeVarKey(pass *analysis.Pass, id *ast.Ident, fn *ast.FuncDecl, file *ast.File) varKey {
	if id == nil {
		return varKey{}
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Defs[id]; obj != nil {
			return varKey{obj: obj, name: id.Name}
		}
		if obj := pass.TypesInfo.Uses[id]; obj != nil {
			return varKey{obj: obj, name: id.Name}
		}
	}
	declPos := findDeclPos(file, fn, id)
	return varKey{declPos: declPos, name: id.Name}
}

// TxState tracks active database transaction instances along an execution path.
type TxState struct {
	activeTxs  map[varKey]bool
	terminated bool
}

func newTxState() *TxState {
	return &TxState{
		activeTxs: make(map[varKey]bool),
	}
}

func (s *TxState) clone() *TxState {
	c := newTxState()
	c.terminated = s.terminated
	for k, v := range s.activeTxs {
		c.activeTxs[k] = v
	}
	return c
}

func (s *TxState) activate(k varKey) {
	if k.name == "" && k.obj == nil && k.declPos == token.NoPos {
		return
	}
	s.activeTxs[k] = true
}

func (s *TxState) deactivate(k varKey) {
	delete(s.activeTxs, k)
}

func (s *TxState) hasActive() bool {
	return len(s.activeTxs) > 0
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
		for k := range st.activeTxs {
			joined.activeTxs[k] = true
		}
	}

	joined.terminated = allTerminated && len(states) > 0
	return joined
}
