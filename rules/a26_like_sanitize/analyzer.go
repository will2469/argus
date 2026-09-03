// Package a26_like_sanitize enforces wildcard sanitization on user input bound to SQL LIKE/ILIKE clauses.
package a26_like_sanitize

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A26.
const RuleCode = "ARGUS-A26"

// Issue describes a detected violation of ARGUS-A26.
type Issue struct {
	Pos     token.Pos
	Message string
}

// Analyzer defines the analysis.Analyzer for ARGUS-A26.
var Analyzer = &analysis.Analyzer{
	Name: "a26",
	Doc:  "Enforce explicit escaping of wildcard characters (%, _, \\) on input bound to LIKE/ILIKE queries (CWE-89, CWE-400)",
	Run:  run,
	Requires: []*analysis.Analyzer{
		callsite.Analyzer,
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

		issues := InspectFile(pass, pass.Fset, file, dm)
		for _, issue := range issues {
			pass.Reportf(issue.Pos, "[%s] %s", RuleCode, issue.Message)
		}
	}

	return nil, nil
}

// InspectFile inspects an AST file for unsanitized wildcard parameters bound to LIKE/ILIKE clauses.
// Can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap) []Issue {
	if file == nil {
		return nil
	}
	var issues []Issue
	var currentFuncBody *ast.BlockStmt

	ast.Inspect(file, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			currentFuncBody = fn.Body
		case *ast.FuncLit:
			currentFuncBody = fn.Body
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		methodName := callsite.GetCallMethodName(call.Fun)
		if !callsite.IsDBQueryMethod(methodName) {
			return true
		}

		if dm != nil && fset != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".LIKE-WILDCARD")) {
			return true
		}

		sql, sqlArgIdx, ok := findQueryAndIndex(call)
		if !ok || sql == "" {
			return true
		}

		likeParamIndices := FindLikeParamIndices(sql)
		if len(likeParamIndices) == 0 {
			return true
		}

		for _, paramIdx := range likeParamIndices {
			argPos := sqlArgIdx + paramIdx
			if argPos < len(call.Args) {
				argExpr := call.Args[argPos]
				if dm != nil && fset != nil && (dm.IsIgnored(fset, argExpr.Pos(), RuleCode) || dm.IsIgnored(fset, argExpr.Pos(), RuleCode+".LIKE-WILDCARD")) {
					continue
				}

				if !IsArgumentSanitized(argExpr, currentFuncBody) {
					issues = append(issues, Issue{
						Pos:     argExpr.Pos(),
						Message: fmt.Sprintf("unsanitized wildcard parameter bound to LIKE/ILIKE clause ($%d); risk of pattern language hijacking, PII exposure, and sequential scan DoS (CWE-89, CWE-400)", paramIdx),
					})
				}
			}
		}

		return true
	})

	return issues
}

func findQueryAndIndex(call *ast.CallExpr) (string, int, bool) {
	if call == nil || len(call.Args) == 0 {
		return "", -1, false
	}

	for idx, arg := range call.Args {
		if s, ok := extractStringLit(arg); ok {
			return s, idx, true
		}
	}
	return "", -1, false
}

func extractStringLit(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			val := e.Value
			if len(val) >= 2 && ((val[0] == '`' && val[len(val)-1] == '`') ||
				(val[0] == '"' && val[len(val)-1] == '"')) {
				return val[1 : len(val)-1], true
			}
		}
	case *ast.Ident:
		if e.Obj != nil {
			switch decl := e.Obj.Decl.(type) {
			case *ast.AssignStmt:
				for i, lhs := range decl.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name == e.Name && i < len(decl.Rhs) {
						return extractStringLit(decl.Rhs[i])
					}
				}
			case *ast.ValueSpec:
				for i, name := range decl.Names {
					if name.Name == e.Name && i < len(decl.Values) {
						return extractStringLit(decl.Values[i])
					}
				}
			}
		}
	}
	return "", false
}
