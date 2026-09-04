// Package a31_mutation_audit_trail traverses interprocedural calls within
// transaction scopes to detect nested database mutations and audit logging.
package a31_mutation_audit_trail

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

type txAuditCollector struct {
	hasNonExemptMutation bool
	hasAudit             bool
	firstMutationPos     ast.Node
}

func (c *txAuditCollector) recordMutation(node ast.Node, isExempt bool) {
	if !isExempt {
		c.hasNonExemptMutation = true
		if c.firstMutationPos == nil {
			c.firstMutationPos = node
		}
	}
}

func (c *txAuditCollector) recordAudit() {
	c.hasAudit = true
}

func walkTxStatements(
	pass *analysis.Pass,
	stmts []ast.Stmt,
	funcDecls []*ast.FuncDecl,
	visited map[*ast.FuncDecl]bool,
	auditMethods []string,
	exemptTables []string,
	collector *txAuditCollector,
) {
	for _, stmt := range stmts {
		ast.Inspect(stmt, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Check if this call is an audit method
			if isAuditCall(call, auditMethods, pass) {
				collector.recordAudit()
				return true
			}

			// Check if this call is a direct mutation
			mutRes := checkMutationCall(call, exemptTables, pass)
			if mutRes.isMutation {
				collector.recordMutation(call, mutRes.isExempt)
				return true
			}

			// Interprocedural traversal into local functions/methods
			targetFn := resolveTargetFunc(pass, call, funcDecls)
			if targetFn != nil && targetFn.Body != nil && !visited[targetFn] {
				visited[targetFn] = true
				defer func() { visited[targetFn] = false }()

				walkTxStatements(pass, targetFn.Body.List, funcDecls, visited, auditMethods, exemptTables, collector)
			}

			return true
		})
	}
}

func resolveTargetFunc(pass *analysis.Pass, call *ast.CallExpr, funcDecls []*ast.FuncDecl) *ast.FuncDecl {
	if call == nil {
		return nil
	}

	// 1. Package-level function: helper()
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

	// 2. Receiver method: receiver.Method()
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

	for _, decl := range funcDecls {
		if decl.Recv != nil && len(decl.Recv.List) > 0 && decl.Name.Name == sel.Sel.Name {
			return decl
		}
	}

	return nil
}
