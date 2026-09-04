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
func CheckAdvisoryHelperArgs(pass *analysis.Pass, fset *token.FileSet, call *ast.CallExpr, body *ast.BlockStmt, dm *directives.DirectiveMap, issues *[]Issue) {
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

		// 4b. Local assignment tracing via exact lexical object identity
		if body != nil {
			var latestVal string
			var found bool
			var isConcat bool
			var concatValid bool

			ast.Inspect(body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || assign.Pos() >= arg.Pos() {
					return true
				}
				for i, lhs := range assign.Lhs {
					targetID, ok := lhs.(*ast.Ident)
					if ok && isSameObject(pass, targetID, id) && i < len(assign.Rhs) {
						rhs := assign.Rhs[i]
						if lit, ok := rhs.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							latestVal = strings.Trim(lit.Value, "`\"")
							found = true
							isConcat = false
						} else if call, ok := rhs.(*ast.CallExpr); ok && isFormatCall(call) {
							if len(call.Args) > 0 {
								if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
									latestVal = strings.Trim(lit.Value, "`\"")
									found = true
									isConcat = false
								}
							}
						} else if bin, ok := rhs.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
							isConcat = true
							concatValid = isNamespacePrefixExpr(bin.X) || isNamespacePrefixExpr(bin.Y)
							if !concatValid {
								if lit, ok := bin.X.(*ast.BasicLit); ok && lit.Kind == token.STRING {
									latestVal = strings.Trim(lit.Value, "`\"")
									found = true
								}
							}
						}
					}
				}
				return true
			})

			if isConcat && concatValid {
				return
			}
			if found {
				checkStringNamespace(arg.Pos(), latestVal, issues)
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
	// Must contain canonical delimiter ':' or '/'
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
