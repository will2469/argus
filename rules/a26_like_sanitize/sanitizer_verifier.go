package a26_like_sanitize

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var trustedSanitizerRegex = regexp.MustCompile(`(?i)//\s*argus:trusted-sanitizer\s+([^\r\n]+)`)

// IsPathologicalLiteral detects static literals causing full table scans or pattern DoS (CWE-400).
func (r *SanitizerRegistry) IsPathologicalLiteral(expr ast.Expr) (bool, string) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false, ""
	}

	val := unquoteString(lit.Value)
	if len(val) == 0 {
		return false, ""
	}

	// 1. Excessive runaway wildcards (e.g. 5+ consecutive wildcards)
	if strings.Contains(val, "%%%%%") {
		return true, fmt.Sprintf("excessive runaway wildcard repetition %q causes sequential scan DoS", val)
	}

	// 2. Pure wildcards ("%", "%%", "%_%") match every row in the table
	if strings.Trim(val, "%_") == "" {
		return true, fmt.Sprintf("pure wildcard %q matches all rows and forces full table scan", val)
	}

	return false, ""
}

func isVerifiedEscapingBody(body *ast.BlockStmt, reg *SanitizerRegistry, pass *analysis.Pass) bool {
	if body == nil {
		return false
	}
	hasPercent := false
	hasUnderscore := false
	callsOtherSanitizer := false

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if reg.IsSanitizerCall(pass, call) {
			callsOtherSanitizer = true
			return false
		}

		// Check for strings.ReplaceAll(s, "%", ...) and strings.ReplaceAll(s, "_", ...)
		if isStringsReplaceCall(call) {
			if len(call.Args) >= 2 {
				if oldLit, ok := extractStringVal(call.Args[1]); ok {
					switch oldLit {
					case "%":
						hasPercent = true
					case "_":
						hasUnderscore = true
					}
				}
			}
		}
		return true
	})

	return callsOtherSanitizer || (hasPercent && hasUnderscore)
}

func isStringsReplaceCall(call *ast.CallExpr) bool {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		name := sel.Sel.Name
		if name == "ReplaceAll" || name == "Replace" {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "strings" {
				return true
			}
		}
	}
	return false
}

func hasTrustedSanitizerDirective(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, comment := range doc.List {
		matches := trustedSanitizerRegex.FindStringSubmatch(comment.Text)
		if len(matches) > 1 {
			reason := strings.TrimSpace(matches[1])
			if len(strings.Fields(reason)) >= 2 {
				return true
			}
		}
	}
	return false
}

func extractStringVal(expr ast.Expr) (string, bool) {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return unquoteString(lit.Value), true
	}
	return "", false
}

func unquoteString(s string) string {
	if len(s) >= 2 && ((s[0] == '`' && s[len(s)-1] == '`') || (s[0] == '"' && s[len(s)-1] == '"')) {
		return s[1 : len(s)-1]
	}
	return s
}

func extractRecvTypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	for {
		if star, ok := expr.(*ast.StarExpr); ok {
			expr = star.X
		} else if paren, ok := expr.(*ast.ParenExpr); ok {
			expr = paren.X
		} else {
			break
		}
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func resolveCallObj(info *types.Info, call *ast.CallExpr) types.Object {
	if info == nil || call == nil {
		return nil
	}
	fun := call.Fun
	for {
		if paren, ok := fun.(*ast.ParenExpr); ok {
			fun = paren.X
		} else {
			break
		}
	}
	switch e := fun.(type) {
	case *ast.Ident:
		if obj, ok := info.Uses[e]; ok && obj != nil {
			return obj
		}
		if obj, ok := info.Defs[e]; ok && obj != nil {
			return obj
		}
	case *ast.SelectorExpr:
		if sel, ok := info.Selections[e]; ok && sel != nil {
			return sel.Obj()
		}
		if obj, ok := info.Uses[e.Sel]; ok && obj != nil {
			return obj
		}
	}
	return nil
}

func extractTypeNameFromType(t types.Type) string {
	if t == nil {
		return ""
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}

