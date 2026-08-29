// Package a09_advisory_lock validates advisory lock helper invocations and namespace hygiene in Go source code.
package a09_advisory_lock

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// CheckAdvisoryHelperArgs inspects Go calls to WithAdvisoryLock and ExecuteLockedTx.
func CheckAdvisoryHelperArgs(pass *analysis.Pass, call *ast.CallExpr, dm *directives.DirectiveMap) {
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

	if lockArg == nil || dm.IsIgnored(pass.Fset, lockArg.Pos(), RuleCode) {
		return
	}

	if lit, ok := lockArg.(*ast.BasicLit); ok && lit.Kind.String() == "STRING" {
		raw := strings.Trim(lit.Value, "`\"")
		if raw == "" {
			pass.Reportf(lockArg.Pos(), "[%s] empty advisory lock name; must provide a namespaced identifier", RuleCode)
		}
	}
}
