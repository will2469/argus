// Package a05_audit_immutability provides path-sensitive flow lattice state
// and object-identity resolution for query variables.
package a05_audit_immutability

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// varKey uniquely identifies a variable either via compiler types.Object
// or via lexical declaration position and name in standalone mode.
type varKey struct {
	obj     types.Object
	declPos token.Pos
	name    string
}

// valueSet represents a set of candidate string values a variable can hold.
// nil represents Unknown (not provably safe), while empty map represents proven empty.
type valueSet map[string]struct{}

func newValueSet(items ...string) valueSet {
	vs := make(valueSet, len(items))
	for _, it := range items {
		vs[it] = struct{}{}
	}
	return vs
}

func (vs valueSet) clone() valueSet {
	res := make(valueSet, len(vs))
	for k := range vs {
		res[k] = struct{}{}
	}
	return res
}

func (vs valueSet) addAll(other valueSet) {
	for k := range other {
		vs[k] = struct{}{}
	}
}

func (vs valueSet) toSlice() []string {
	if len(vs) == 0 {
		return nil
	}
	res := make([]string, 0, len(vs))
	for k := range vs {
		res = append(res, k)
	}
	return res
}

// flowState tracks reaching value sets for variables at a given program point.
type flowState struct {
	values map[varKey]valueSet
}

func newFlowState() *flowState {
	return &flowState{
		values: make(map[varKey]valueSet),
	}
}

func (s *flowState) clone() *flowState {
	c := newFlowState()
	for k, v := range s.values {
		// Preserve nil (Unknown state) during clone
		if v == nil {
			c.values[k] = nil
		} else {
			c.values[k] = v.clone()
		}
	}
	return c
}

func (s *flowState) set(k varKey, vs valueSet) {
	// Preserve nil (Unknown state) during set
	if vs == nil {
		s.values[k] = nil
	} else {
		s.values[k] = vs.clone()
	}
}

func (s *flowState) get(k varKey) (valueSet, bool) {
	v, ok := s.values[k]
	return v, ok
}

// join merges two branch states using a union lattice join.
// Fail-closed: if either branch has Unknown (nil) for a variable, the result is Unknown.
func (s *flowState) join(other *flowState) *flowState {
	if other == nil {
		return s.clone()
	}
	merged := s.clone()
	for k, v := range other.values {
		if existing, ok := merged.values[k]; ok {
			// If either branch is Unknown (nil), result is Unknown (fail-closed)
			if existing == nil || v == nil {
				merged.values[k] = nil
			} else {
				// Both are known (either empty or have values) - union them
				existing.addAll(v)
			}
		} else {
			// Copy the other branch's value (including nil for Unknown)
			if v == nil {
				merged.values[k] = nil
			} else {
				merged.values[k] = v.clone()
			}
		}
	}
	return merged
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

func findDeclPos(id *ast.Ident, fn *ast.FuncDecl, file *ast.File) token.Pos {
	if id == nil {
		return token.NoPos
	}

	// 1. Search local enclosing blocks from innermost to outermost
	if fn != nil && fn.Body != nil {
		blocks := getEnclosingBlocks(fn.Body, id.Pos())
		for i := len(blocks) - 1; i >= 0; i-- {
			b := blocks[i]
			for _, stmt := range b.List {
				if stmt.Pos() >= id.Pos() {
					continue
				}
				switch s := stmt.(type) {
				case *ast.AssignStmt:
					if s.Tok == token.DEFINE {
						for _, lhs := range s.Lhs {
							if ident, ok := lhs.(*ast.Ident); ok && ident.Name == id.Name {
								return ident.Pos()
							}
						}
					}
				case *ast.DeclStmt:
					if gen, ok := s.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
						for _, spec := range gen.Specs {
							if valSpec, ok := spec.(*ast.ValueSpec); ok {
								for _, name := range valSpec.Names {
									if name.Name == id.Name {
										return name.Pos()
									}
								}
							}
						}
					}
				}
			}
		}

		// 2. Search function parameters
		if fn.Type != nil && fn.Type.Params != nil {
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					if name.Name == id.Name {
						return name.Pos()
					}
				}
			}
		}
	}

	// 3. Search package-level declarations in file
	if file != nil {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range gen.Specs {
					if valSpec, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range valSpec.Names {
							if name.Name == id.Name {
								return name.Pos()
							}
						}
					}
				}
			}
		}
	}

	return token.NoPos
}

func getEnclosingBlocks(root ast.Node, pos token.Pos) []*ast.BlockStmt {
	if root == nil {
		return nil
	}
	var blocks []*ast.BlockStmt
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil || n.Pos() > pos || n.End() < pos {
			return false
		}
		if b, ok := n.(*ast.BlockStmt); ok {
			blocks = append(blocks, b)
		}
		return true
	})
	return blocks
}
