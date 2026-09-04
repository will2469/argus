// Package a09_advisory_lock validates advisory lock helper invocations and namespace hygiene in Go source code,
// ensuring that lock keys are qualified with structured domain namespaces (e.g. "domain:resource").
package a09_advisory_lock

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// CheckAdvisoryHelperArgs inspects Go calls to WithAdvisoryLock, ExecuteLockedTx, and TryAdvisoryLock.
func CheckAdvisoryHelperArgs(pass *analysis.Pass, fset *token.FileSet, call *ast.CallExpr, fn *ast.FuncDecl, file *ast.File, dm *directives.DirectiveMap, issues *[]Issue) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	// Verify that the method name is an approved advisory lock helper
	if !isAdvisoryLockHelperMethod(sel) {
		return
	}

	// Verify receiver approval via semantic and AST contract analysis
	if !isAdvisoryLockHelperReceiver(pass, sel, fn, file) {
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

	var body *ast.BlockStmt
	if fn != nil {
		body = fn.Body
	}
	validateLockIdentifier(pass, lockArg, body, issues)
}

func validateLockIdentifier(pass *analysis.Pass, arg ast.Expr, body *ast.BlockStmt, issues *[]Issue) {
	// 1. Direct string literal
	if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		raw := strings.Trim(lit.Value, "`\"")
		checkStringNamespace(arg.Pos(), raw, issues)
		return
	}

	// 2. Direct fmt.Sprintf call: fmt.Sprintf("orders:%s", id)
	if call, ok := arg.(*ast.CallExpr); ok && isFormatCall(call) {
		checkFormatCall(arg.Pos(), call, issues)
		return
	}

	// 3. String concatenation: "orders:" + id
	if bin, ok := arg.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		if isNamespacePrefixExpr(bin.X) || isNamespacePrefixExpr(bin.Y) {
			return
		}
		if lit, ok := bin.X.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			raw := strings.Trim(lit.Value, "`\"")
			checkStringNamespace(arg.Pos(), raw, issues)
			return
		}
	}

	// 4. Identifier: Constant resolution & object-identity assignment tracing
	if id, ok := arg.(*ast.Ident); ok {
		// 4a. Check string constant
		if constVal, ok := resolveStringConstant(pass, id); ok {
			checkStringNamespace(arg.Pos(), constVal, issues)
			return
		}

		// 4b. Local assignment tracing via scope-hierarchy object identity
		if body != nil {
			var assignedVals []string
			var hasConcat bool
			var concatValid bool

			ast.Inspect(body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || assign.Pos() >= arg.Pos() {
					return true
				}
				for i, lhs := range assign.Lhs {
					targetID, ok := lhs.(*ast.Ident)
					if ok && isSameObject(pass, targetID, id, body) && i < len(assign.Rhs) {
						rhs := assign.Rhs[i]
						if lit, ok := rhs.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							assignedVals = append(assignedVals, strings.Trim(lit.Value, "`\""))
						} else if call, ok := rhs.(*ast.CallExpr); ok && isFormatCall(call) {
							if len(call.Args) > 0 {
								if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
									assignedVals = append(assignedVals, strings.Trim(lit.Value, "`\""))
								}
							}
						} else if bin, ok := rhs.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
							hasConcat = true
							if isNamespacePrefixExpr(bin.X) || isNamespacePrefixExpr(bin.Y) {
								concatValid = true
							} else if lit, ok := bin.X.(*ast.BasicLit); ok && lit.Kind == token.STRING {
								assignedVals = append(assignedVals, strings.Trim(lit.Value, "`\""))
							}
						}
					}
				}
				return true
			})

			if hasConcat && !concatValid && len(assignedVals) == 0 {
				*issues = append(*issues, Issue{
					Pos:     arg.Pos(),
					Message: fmt.Sprintf("unnamespaced advisory lock identifier %q; lock keys must use a structured namespace format (e.g. \"domain:resource\")", id.Name),
				})
				return
			}

			// Fail-closed lattice: If any reaching assigned value is unnamespaced, report violation
			for _, val := range assignedVals {
				if !isStructuredNamespace(val) {
					checkStringNamespace(arg.Pos(), val, issues)
					return
				}
			}
		}
	}
}

func checkStringNamespace(pos token.Pos, raw string, issues *[]Issue) {
	if raw == "" {
		*issues = append(*issues, Issue{
			Pos:     pos,
			Message: "empty advisory lock name; must provide a namespaced identifier",
		})
		return
	}

	if !isStructuredNamespace(raw) {
		*issues = append(*issues, Issue{
			Pos:     pos,
			Message: fmt.Sprintf("unnamespaced advisory lock identifier %q; lock keys must use a structured namespace format (e.g. \"domain:resource\" or fmt.Sprintf(\"orders:%%s\", id))", raw),
		})
	}
}

func isStructuredNamespace(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	// Structured namespaces must contain canonical delimiter ':' or '/'
	// Delimiters like '.' (e.g. "foo.bar") are rejected as unnamespaced.
	delimIdx := strings.IndexAny(raw, ":/")
	if delimIdx <= 0 || delimIdx >= len(raw)-1 {
		return false
	}
	domain := strings.TrimSpace(raw[:delimIdx])
	resource := strings.TrimSpace(raw[delimIdx+1:])
	return domain != "" && resource != ""
}

func isNamespacePrefixExpr(expr ast.Expr) bool {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		raw := strings.Trim(lit.Value, "`\"")
		raw = strings.TrimSpace(raw)
		delimIdx := strings.IndexAny(raw, ":/")
		return delimIdx > 0
	}
	return false
}

func checkFormatCall(pos token.Pos, call *ast.CallExpr, issues *[]Issue) {
	if len(call.Args) > 0 {
		if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
			raw := strings.Trim(lit.Value, "`\"")
			checkStringNamespace(pos, raw, issues)
		}
	}
}

func isFormatCall(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "fmt" {
		return sel.Sel.Name == "Sprintf"
	}
	return false
}

// isAdvisoryLockHelperMethod checks if the selector expression refers to an approved helper method name.
func isAdvisoryLockHelperMethod(sel *ast.SelectorExpr) bool {
	if sel == nil {
		return false
	}
	switch sel.Sel.Name {
	case "WithAdvisoryLock", "ExecuteLockedTx", "TryAdvisoryLock":
		return true
	}
	return false
}
