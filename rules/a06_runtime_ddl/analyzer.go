// Package a06_runtime_ddl prohibits executing DDL statements in Go application runtime code.
package a06_runtime_ddl

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A06"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A06.
var Analyzer = &analysis.Analyzer{
	Name: "a06",
	Doc:  "Prohibits execution of DDL statements (CREATE, ALTER, DROP, TRUNCATE, GRANT, REVOKE) in runtime Go application code",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A06.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for runtime DDL execution.
// Can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap) []Issue {
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

	tracker := NewDDLTracker(pass, file)
	tracker.Analyze()

	var issues []Issue
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		tracker.SetCurrentFunc(fn)

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

			// Identify the actual SQL query argument, ignoring bound parameter arguments
			sqlArg := callsite.ExtractSQLArg(call, pass)
			if sqlArg == nil {
				return true
			}

			if dm != nil && fset != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, sqlArg.Pos(), RuleCode)) {
				return true
			}

			// 1. Static AST detection on direct literal query strings
			if lit, ok := sqlArg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				val := strings.Trim(lit.Value, "`\"")
				if ddlOp := DetectDDLFromAST(val); ddlOp != "" {
					issues = append(issues, Issue{
						Pos:     call.Pos(),
						Message: fmt.Sprintf("runtime database query contains forbidden DDL statement (%s); DDL is restricted to migrations", ddlOp),
					})
					return true
				}
			}

			// 2. Flow-sensitive DDL detection evaluated strictly on the SQL argument expression
			if ddlOp := tracker.GetDDLOpAt(sqlArg, call); ddlOp != "" {
				issues = append(issues, Issue{
					Pos:     call.Pos(),
					Message: fmt.Sprintf("runtime database query contains forbidden DDL statement (%s); DDL is restricted to migrations", ddlOp),
				})
				return true
			}

			return true
		})
	}

	return issues
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

	recvName := ""
	if id, ok := sel.X.(*ast.Ident); ok {
		recvName = strings.ToLower(id.Name)
	}

	if isNonDatabaseReceiverName(recvName) {
		return false
	}

	return true
}

func isNonDatabaseReceiverName(name string) bool {
	switch {
	case strings.Contains(name, "logger"), strings.Contains(name, "log"),
		strings.Contains(name, "slog"), strings.Contains(name, "zap"),
		strings.Contains(name, "http"), strings.Contains(name, "client"),
		strings.Contains(name, "queue"), strings.Contains(name, "search"),
		strings.Contains(name, "searchengine"), strings.Contains(name, "cmd"),
		strings.Contains(name, "runner"), strings.Contains(name, "cache"):
		return true
	}
	return false
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	for _, file := range pass.Files {
		issues := InspectFile(pass, pass.Fset, file, dm)
		for _, iss := range issues {
			pass.Reportf(iss.Pos, "[%s] %s", RuleCode, iss.Message)
		}
	}

	return nil, nil
}
