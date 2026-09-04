// Package a04_orderby detects unsafe dynamic ORDER BY clauses and enforces
// that sort columns and directions originate from closed-set allowlist maps or switch-case branches.
package a04_orderby

import (
	"fmt"
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
	Name: "a04",
	Doc:  "Enforces closed-set allowlist map or switch-case validation for dynamic ORDER BY clauses",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A04.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for unsafe dynamic ORDER BY clauses with default config.
// Can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap) []Issue {
	return InspectFileWithConfig(pass, fset, file, dm, nil)
}

// InspectFileWithConfig inspects an AST file for unsafe dynamic ORDER BY clauses with optional allowed_columns.
func InspectFileWithConfig(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap, allowedCols []string) []Issue {
	if file == nil {
		return nil
	}
	if fset == nil && pass != nil {
		fset = pass.Fset
	}

	pos := fset.Position(file.Package)
	if strings.HasSuffix(pos.Filename, "_test.go") {
		return nil
	}

	var issues []Issue
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		inspectFunctionBody(pass, fset, file, fn.Body, dm, allowedCols, &issues)
	}
	return issues
}

func inspectFunctionBody(pass *analysis.Pass, fset *token.FileSet, file *ast.File, body *ast.BlockStmt, dm *directives.DirectiveMap, allowedCols []string, issues *[]Issue) {
	ast.Inspect(body, func(n ast.Node) bool {
		// Inspect nested closures separately
		if lit, ok := n.(*ast.FuncLit); ok && lit.Body != nil {
			inspectFunctionBody(pass, fset, file, lit.Body, dm, allowedCols, issues)
			return false
		}

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
						checkOrderByArg(pass, fset, file, call.Args[idx], call.Pos(), body, dm, allowedCols, issues)
					}
				}
			}
		}

		return true
	})
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	allowedCols := cfg.GetStringSlice(RuleCode, "allowed_columns", nil)
	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	for _, file := range pass.Files {
		issues := InspectFileWithConfig(pass, pass.Fset, file, dm, allowedCols)
		for _, iss := range issues {
			pass.Reportf(iss.Pos, "[%s] %s", RuleCode, iss.Message)
		}
	}

	return nil, nil
}

func checkOrderByArg(pass *analysis.Pass, fset *token.FileSet, file *ast.File, arg ast.Expr, callPos token.Pos, body *ast.BlockStmt, dm *directives.DirectiveMap, allowedCols []string, issues *[]Issue) {
	if fset != nil && dm != nil {
		if dm.IsIgnored(fset, arg.Pos(), RuleCode) || dm.IsIgnored(fset, callPos, RuleCode) {
			return
		}
	}

	// Direct string literals are safe constants
	if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return
	}

	// Quoting sanitizer pgx.Identifier.Sanitize() is explicitly rejected for ORDER BY
	if IsQuotingSanitizer(arg) {
		*issues = append(*issues, Issue{
			Pos:     arg.Pos(),
			Message: "identifier quoting is insufficient for ORDER BY; must be mapped via closed-set allowlist map or switch-case",
		})
		return
	}

	// Check variable data flow
	if ident, ok := arg.(*ast.Ident); ok {
		if !IsSafeOrderBy(ident, body, file, pass, allowedCols) && !IsSortDirectionSafe(ident, body) {
			*issues = append(*issues, Issue{
				Pos:     arg.Pos(),
				Message: fmt.Sprintf("unsafe dynamic ORDER BY variable %q; must be mapped via closed-set allowlist map or switch-case", ident.Name),
			})
		}
		return
	}

	// Any other dynamic expression (e.g. r.URL.Query().Get("sort")) is unsafe
	*issues = append(*issues, Issue{
		Pos:     arg.Pos(),
		Message: "unsafe dynamic ORDER BY expression; must be mapped via closed-set allowlist map or switch-case",
	})
}
