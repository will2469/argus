// Package a09_advisory_lock validates advisory lock helper invocations and namespace hygiene in Go source code.
package a09_advisory_lock

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// CheckAdvisoryHelperArgs inspects Go calls to WithAdvisoryLock and ExecuteLockedTx.
func CheckAdvisoryHelperArgs(pass *analysis.Pass, fset *token.FileSet, call *ast.CallExpr, dm *directives.DirectiveMap, issues *[]Issue) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	var lockArg ast.Expr
	switch sel.Sel.Name {
	case "WithAdvisoryLock":
		// WithAdvisoryLock(ctx, tx, lockName, failFast, fn) -> arg index 2
		if len(call.Args) >= 3 {
			lockArg = call.Args[2]
		}
	case "ExecuteLockedTx":
		// ExecuteLockedTx(ctx, pool, lockName, fn) -> arg index 2
		if len(call.Args) >= 3 {
			lockArg = call.Args[2]
		}
	case "TryAdvisoryLock":
		// TryAdvisoryLock(ctx, pool, lockName) -> arg index 2
		if len(call.Args) >= 3 {
			lockArg = call.Args[2]
		}
	}

	if lockArg == nil || (fset != nil && dm != nil && dm.IsIgnored(fset, lockArg.Pos(), RuleCode)) {
		return
	}

	if lit, ok := lockArg.(*ast.BasicLit); ok && lit.Kind.String() == "STRING" {
		raw := strings.Trim(lit.Value, "`\"")
		if raw == "" {
			*issues = append(*issues, Issue{
				Pos:     lockArg.Pos(),
				Message: "empty advisory lock name; must provide a namespaced identifier",
			})
		}
	}
}
