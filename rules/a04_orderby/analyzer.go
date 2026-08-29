// Package a04_orderby detects unsafe dynamic ORDER BY clauses and enforces
// that sort columns and directions originate from closed-set allowlist maps or switch-case branches.
package a04_orderby

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A04"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A04.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a04_unsafe_order_by",
	Doc:  "Enforces closed-set allowlist map or switch-case validation for dynamic ORDER BY clauses",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	for _, file := range pass.Files {
		pos := pass.Fset.Position(file.Package)
		if strings.HasSuffix(pos.Filename, "_test.go") {
			continue
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				if IsFmtSprintf(call) && len(call.Args) >= 2 {
					formatLit, ok := call.Args[0].(*ast.BasicLit)
					if ok && formatLit.Kind == token.STRING && HasOrderByClause(formatLit.Value) {
						rawFormat, err := strconv.Unquote(formatLit.Value)
						if err != nil {
							rawFormat = strings.Trim(formatLit.Value, "`\"")
						}
						targetIndices := GetOrderByArgIndices(rawFormat)
						for _, idx := range targetIndices {
							if idx < len(call.Args) {
								checkOrderByArg(pass, call.Args[idx], call.Pos(), fn.Body, dm)
							}
						}
					}
				}

				return true
			})
		}
	}

	return nil, nil
}

func checkOrderByArg(pass *analysis.Pass, arg ast.Expr, callPos token.Pos, body *ast.BlockStmt, dm *directives.DirectiveMap) {
	if dm.IsIgnored(pass.Fset, arg.Pos(), RuleCode) || dm.IsIgnored(pass.Fset, callPos, RuleCode) {
		return
	}

	// Direct string literals are safe constants
	if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return
	}

	// Quoting sanitizer pgx.Identifier.Sanitize() is explicitly rejected for ORDER BY
	if IsQuotingSanitizer(arg) {
		pass.Reportf(arg.Pos(), "[%s] identifier quoting is insufficient for ORDER BY; must be mapped via closed-set allowlist map or switch-case", RuleCode)
		return
	}

	// Check variable data flow
	if ident, ok := arg.(*ast.Ident); ok {
		if !IsSafeOrderBy(ident, body) && !IsSortDirectionSafe(ident, body) {
			pass.Reportf(arg.Pos(), "[%s] unsafe dynamic ORDER BY variable %q; must be mapped via closed-set allowlist map or switch-case", RuleCode, ident.Name)
		}
		return
	}

	// Any other dynamic expression (e.g. r.URL.Query().Get("sort")) is unsafe
	pass.Reportf(arg.Pos(), "[%s] unsafe dynamic ORDER BY expression; must be mapped via closed-set allowlist map or switch-case", RuleCode)
}
