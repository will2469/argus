// Package a01_sql_concat prohibits dynamic SQL concatenation and string formatting
// in favor of compile-time string constants with parameterized placeholders ($1, $2, ...).
package a01_sql_concat

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A01"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A01.
var Analyzer = &analysis.Analyzer{
	Name: "a01",
	Doc:  "Prohibits dynamic SQL concatenation and formatting without parameterization or identifier sanitization",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A01.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for dynamic SQL concatenation or formatting.
// It can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap) []Issue {
	return InspectFileWithConfig(pass, fset, file, dm, nil)
}

// InspectFileWithConfig inspects an AST file using optional custom taint sources (from .argus.yaml).
func InspectFileWithConfig(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap, customSources []string) []Issue {
	tracker := NewTaintTracker(pass, file)
	tracker.SetCustomTaintSources(customSources)
	tracker.Analyze()
	return InspectFileWithTracker(pass, fset, file, dm, tracker)
}

// InspectFileWithTracker inspects an AST file using a pre-initialized TaintTracker.
func InspectFileWithTracker(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap, tracker *TaintTracker) []Issue {
	var issues []Issue
	if file == nil {
		return nil
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if tracker != nil {
			tracker.SetCurrentFunc(fn)
		}

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
				return true
			}

			if !isDatabaseCall(pass, call, sel) {
				return true
			}

			sqlArg := extractSQLArgument(call)
			if sqlArg == nil {
				return true
			}

			if dm != nil && fset != nil && dm.IsIgnored(fset, sqlArg.Pos(), RuleCode) {
				return true
			}

			if isUnsafeSQL(sqlArg, tracker, pass, call) {
				issues = append(issues, Issue{
					Pos:     sqlArg.Pos(),
					Message: "unsafe SQL concatenation or formatting; use parameterized placeholders ($1, $2, ...) or SanitizeIdentifier",
				})
			}
			return true
		})
	}
	if tracker != nil {
		tracker.SetCurrentFunc(nil)
	}

	return issues
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)
	customSources := cfg.GetStringSlice(RuleCode, "custom_taint_sources", nil)

	tracker := NewTaintTracker(pass, pass.Files...)
	tracker.SetCustomTaintSources(customSources)
	tracker.Analyze()

	for _, file := range pass.Files {
		issues := InspectFileWithTracker(pass, pass.Fset, file, dm, tracker)
		for _, issue := range issues {
			pass.Reportf(issue.Pos, "[%s] %s", RuleCode, issue.Message)
		}
	}

	return nil, nil
}

func isDatabaseCall(pass *analysis.Pass, call *ast.CallExpr, sel *ast.SelectorExpr) bool {
	if call == nil || sel == nil {
		return false
	}

	if pass != nil && pass.TypesInfo != nil {
		var recv types.Type
		if selType, ok := pass.TypesInfo.Selections[sel]; ok && selType.Recv() != nil {
			recv = selType.Recv()
		} else if tv, ok := pass.TypesInfo.Types[sel.X]; ok && tv.Type != nil {
			recv = tv.Type
		}
		if recv != nil {
			if callsite.IsPgxOrSQLType(recv) {
				return true
			}
			if isNonDatabaseReceiverName(strings.ToLower(recv.String())) {
				return false
			}
		}
	}

	if hasSQLArgument(call) {
		return true
	}

	recvName := ""
	if id, ok := sel.X.(*ast.Ident); ok {
		recvName = strings.ToLower(id.Name)
	}

	if isNonDatabaseReceiverName(recvName) {
		return false
	}

	return isDatabaseReceiverName(recvName)
}

func isNonDatabaseReceiverName(name string) bool {
	switch {
	case strings.Contains(name, "logger"), strings.Contains(name, "log"),
		strings.Contains(name, "slog"), strings.Contains(name, "zap"),
		strings.Contains(name, "http"), strings.Contains(name, "client"),
		strings.Contains(name, "queue"), strings.Contains(name, "search"),
		strings.Contains(name, "searchengine"), strings.Contains(name, "cmd"):
		return true
	}
	return false
}

func isDatabaseReceiverName(name string) bool {
	switch name {
	case "db", "pool", "tx", "conn", "queries", "querier", "database", "store", "repo", "repository", "r", "exec":
		return true
	}
	for _, suffix := range []string{"db", "pool", "tx", "conn", "repo", "store", "exec"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func isUnsafeSQL(e ast.Expr, tracker *TaintTracker, pass *analysis.Pass, at ast.Node) bool {
	if e == nil {
		return false
	}
	if IsSanitized(e, pass) {
		return false
	}

	switch x := e.(type) {
	case *ast.BasicLit:
		return false // Pure string literal is safe
	case *ast.ParenExpr:
		return isUnsafeSQL(x.X, tracker, pass, at)
	case *ast.StarExpr:
		return isUnsafeSQL(x.X, tracker, pass, at)
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return !IsSafeConcat(x, pass)
		}
	case *ast.CallExpr:
		if IsFormattingCall(x, tracker, pass) {
			return true
		}
		if IsBuilderString(x) {
			if tracker != nil {
				return tracker.IsTaintedAt(x, at)
			}
			return true
		}
	case *ast.Ident:
		if pass != nil && pass.TypesInfo != nil {
			if obj := pass.TypesInfo.Uses[x]; obj != nil {
				if _, ok := obj.(*types.Const); ok {
					return false
				}
			}
		}
		if tracker != nil {
			return tracker.IsTaintedAt(x, at)
		}
		return true
	}

	return false
}
