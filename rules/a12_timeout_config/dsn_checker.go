package a12_timeout_config

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"net/url"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/directives"
)

// DSNCheckResult holds missing and invalid timeout parameters found in a DSN.
type DSNCheckResult struct {
	Missing []string
	Zero    []string
}

// CheckDSN evaluates a PostgreSQL connection string for required timeout parameters.
func CheckDSN(dsn string) DSNCheckResult {
	var result DSNCheckResult

	u, err := url.Parse(dsn)
	var q url.Values
	if err == nil {
		q = u.Query()
	}

	checkParam := func(name string, aliases ...string) {
		val := ""
		found := false

		if q != nil {
			if v := q.Get(name); v != "" {
				val = v
				found = true
			} else {
				for _, alias := range aliases {
					if v := q.Get(alias); v != "" {
						val = v
						found = true
						break
					}
				}
			}
		}

		if !found {
			// Fallback text check for key=value pairs in non-URL DSNs
			for _, key := range append([]string{name}, aliases...) {
				if v, ok := findKVParam(dsn, key); ok {
					val = v
					found = true
					break
				}
			}
		}

		if !found {
			result.Missing = append(result.Missing, name)
		} else if isZeroValue(val) {
			result.Zero = append(result.Zero, name)
		}
	}

	checkParam("statement_timeout")
	checkParam("lock_timeout")
	checkParam("idle_in_transaction_session_timeout", "idle_in_transaction")

	return result
}

func checkDSNCall(fset *token.FileSet, dsnExpr ast.Expr, dsn string, dm *directives.DirectiveMap, issues *[]Issue) {
	if fset != nil && dm != nil && dm.IsIgnored(fset, dsnExpr.Pos(), RuleCode) {
		return
	}

	res := CheckDSN(dsn)
	for _, missing := range res.Missing {
		*issues = append(*issues, Issue{
			Pos:     dsnExpr.Pos(),
			Message: fmt.Sprintf("pgxpool DSN missing '%s' parameter; add '%s=<duration>' to prevent unbounded resource consumption", missing, missing),
		})
	}
	for _, zero := range res.Zero {
		*issues = append(*issues, Issue{
			Pos:     dsnExpr.Pos(),
			Message: fmt.Sprintf("pgxpool DSN parameter '%s' must not be set to 0 (unlimited)", zero),
		})
	}
}

func extractAllDSNStrings(call *ast.CallExpr, file *ast.File, pass *analysis.Pass) []string {
	if call == nil || len(call.Args) == 0 {
		return nil
	}
	arg, _ := findCallArg(call, pass)
	if arg == nil {
		return nil
	}

	var results []string
	switch e := arg.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			results = append(results, strings.Trim(e.Value, "`\""))
		}
	case *ast.Ident:
		enclosing := findEnclosingFunc(file, call.Pos())
		if enclosing != nil && enclosing.Body != nil {
			var targetObj types.Object
			if pass != nil && pass.TypesInfo != nil {
				targetObj = pass.TypesInfo.Uses[e]
				if targetObj == nil {
					targetObj = pass.TypesInfo.Defs[e]
				}
			}
			ast.Inspect(enclosing.Body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || assign.Pos() >= call.Pos() {
					return true
				}
				for i, lhs := range assign.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && i < len(assign.Rhs) {
						match := false
						if pass != nil && pass.TypesInfo != nil && targetObj != nil {
							if obj := pass.TypesInfo.Defs[id]; obj != nil && obj == targetObj {
								match = true
							} else if obj := pass.TypesInfo.Uses[id]; obj != nil && obj == targetObj {
								match = true
							}
						} else if id.Name == e.Name {
							match = true
						}
						if match {
							if lit, ok := assign.Rhs[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								results = append(results, strings.Trim(lit.Value, "`\""))
							}
						}
					}
				}
				return true
			})
		}
	}

	if len(results) == 0 {
		if s, ok := callsite.ExtractQueryString(call); ok {
			results = append(results, s)
		}
	}
	return results
}

func findKVParam(dsn, key string) (string, bool) {
	lowerDSN := strings.ToLower(dsn)
	target := strings.ToLower(key) + "="
	idx := strings.Index(lowerDSN, target)
	if idx == -1 {
		return "", false
	}
	start := idx + len(target)
	rest := dsn[start:]
	end := strings.IndexAny(rest, " &;\n\r\t")
	if end == -1 {
		return rest, true
	}
	return rest[:end], true
}

func isZeroValue(val string) bool {
	v := strings.Trim(val, `"' `)
	v = strings.ToLower(v)
	return v == "0" || v == "0s" || v == "0ms" || v == "0m"
}
