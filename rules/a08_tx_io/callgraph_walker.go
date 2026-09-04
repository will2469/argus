// Package a08_tx_io traverses interprocedural calls inside transaction bodies to detect indirect blocking I/O.
package a08_tx_io

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/directives"
)

// CheckTxNode inspects a node inside a transaction body, directly or by walking helper function definitions.
func CheckTxNode(pass *analysis.Pass, fset *token.FileSet, n ast.Node, fn *ast.FuncDecl, file *ast.File, funcDecls []*ast.FuncDecl, visited map[*ast.FuncDecl]bool, dm *directives.DirectiveMap, issues *[]Issue) {
	// 1. Direct blocking I/O on node
	if op := CheckBlockingIOWithContext(n, pass, fn, file); op != "" {
		if fset != nil && dm != nil && dm.IsIgnored(fset, n.Pos(), RuleCode) {
			return
		}
		*issues = append(*issues, Issue{
			Pos:     n.Pos(),
			Message: fmt.Sprintf("blocking external I/O (%s) detected inside database transaction; holding open transactions causes connection pool starvation and lock cascade", op),
		})
		return
	}

	// 2. Interprocedural call traversal to local functions
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return
	}

	targetFn := resolveTargetFunc(pass, call, fn, file, funcDecls)
	if targetFn == nil || targetFn.Body == nil || visited[targetFn] {
		return
	}

	visited[targetFn] = true
	defer func() { visited[targetFn] = false }()

	ast.Inspect(targetFn.Body, func(innerNode ast.Node) bool {
		if innerNode == nil {
			return true
		}
		if op := CheckBlockingIOWithContext(innerNode, pass, targetFn, file); op != "" {
			if fset != nil && dm != nil {
				if dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, innerNode.Pos(), RuleCode) {
					return true
				}
			}
			*issues = append(*issues, Issue{
				Pos:     call.Pos(),
				Message: fmt.Sprintf("blocking external I/O (%s) detected inside database transaction; holding open transactions causes connection pool starvation and lock cascade", op),
			})
		}
		return true
	})
}

func resolveTargetFunc(pass *analysis.Pass, call *ast.CallExpr, enclosingFn *ast.FuncDecl, file *ast.File, funcDecls []*ast.FuncDecl) *ast.FuncDecl {
	// 1. Package-level function call: helper()
	if id, ok := call.Fun.(*ast.Ident); ok {
		if pass != nil && pass.TypesInfo != nil {
			if fnObj, ok := pass.TypesInfo.Uses[id].(*types.Func); ok {
				for _, decl := range funcDecls {
					if pass.TypesInfo.Defs[decl.Name] == fnObj {
						return decl
					}
				}
			}
		}
		for _, decl := range funcDecls {
			if decl.Recv == nil && decl.Name.Name == id.Name {
				return decl
			}
		}
		return nil
	}

	// 2. Receiver method call: receiver.Method()
	sel := callsite.GetCallSelector(call.Fun)
	if sel == nil {
		return nil
	}

	if pass != nil && pass.TypesInfo != nil {
		if selType, ok := pass.TypesInfo.Selections[sel]; ok {
			if fnObj, ok := selType.Obj().(*types.Func); ok {
				for _, decl := range funcDecls {
					if pass.TypesInfo.Defs[decl.Name] == fnObj {
						return decl
					}
				}
			}
		}
	}

	astType := findASTType(sel.X, enclosingFn, file)
	recvTypeName := getASTTypeName(astType)
	if recvTypeName == "" {
		return nil
	}

	for _, decl := range funcDecls {
		if decl.Recv != nil && len(decl.Recv.List) > 0 {
			declRecvType := getASTTypeName(decl.Recv.List[0].Type)
			if declRecvType == recvTypeName && decl.Name.Name == sel.Sel.Name {
				return decl
			}
		}
	}

	return nil
}
