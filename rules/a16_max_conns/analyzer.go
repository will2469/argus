// Package a16_max_conns enforces explicit, bounded MaxConns configuration on pgxpool
// to prevent Linux kernel process thrashing, memory exhaustion, and connection starvation.
package a16_max_conns

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A16.
const RuleCode = "ARGUS-A16"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A16.
var Analyzer = &analysis.Analyzer{
	Name: "a16",
	Doc:  "Enforce explicit, bounded MaxConns configuration on pgxpool to prevent process thrashing and connection exhaustion",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A16.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for unbounded or missing MaxConns in pgxpool initialization.
// Supports both pass mode and standalone runner mode (pass == nil).
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap, maxSafe int32) []Issue {
	if file == nil {
		return nil
	}
	if fset == nil && pass != nil {
		fset = pass.Fset
	}
	if maxSafe <= 0 {
		maxSafe = DefaultMaxSafeConns
	}

	pos := fset.Position(file.Package)
	if strings.HasSuffix(pos.Filename, "_test.go") {
		return nil
	}

	var issues []Issue
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		inspectCall(fset, call, file, dm, maxSafe, &issues)
		return true
	})

	return issues
}

func inspectCall(fset *token.FileSet, call *ast.CallExpr, file *ast.File, dm *directives.DirectiveMap, maxSafe int32, issues *[]Issue) {
	methodName := callsite.GetCallMethodName(call.Fun)

	switch methodName {
	case "New", "pgxpool.New":
		if !isPgxPoolCall(call.Fun) {
			return
		}
		dsnArgIdx := 0
		if len(call.Args) >= 2 {
			dsnArgIdx = 1
		}
		if dsnArgIdx < len(call.Args) {
			dsnStrings := extractAllDSNStrings(call, file)
			for _, dsnStr := range dsnStrings {
				checkDSNCall(fset, call.Args[dsnArgIdx], dsnStr, dm, maxSafe, issues)
			}
		}
	case "NewWithConfig", "pgxpool.NewWithConfig":
		cfgArgIdx := 0
		if len(call.Args) >= 2 {
			cfgArgIdx = 1
		}
		if cfgArgIdx < len(call.Args) {
			checkNewWithConfigCall(fset, call, call.Args[cfgArgIdx], file, dm, maxSafe, issues)
		}
	}
}

func extractAllDSNStrings(call *ast.CallExpr, file *ast.File) []string {
	if call == nil || len(call.Args) == 0 {
		return nil
	}
	dsnArgIdx := 0
	if len(call.Args) >= 2 {
		dsnArgIdx = 1
	}
	arg := call.Args[dsnArgIdx]

	var results []string
	switch e := arg.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			results = append(results, strings.Trim(e.Value, "`\""))
		}
	case *ast.Ident:
		enclosing := findEnclosingFunc(file, call)
		if enclosing != nil && enclosing.Body != nil {
			ast.Inspect(enclosing.Body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok || assign.Pos() >= call.Pos() {
					return true
				}
				for i, lhs := range assign.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name == e.Name && i < len(assign.Rhs) {
						if lit, ok := assign.Rhs[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							results = append(results, strings.Trim(lit.Value, "`\""))
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

func isPgxPoolCall(fn ast.Expr) bool {
	sel, ok := fn.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkgName := ""
	if id, ok := sel.X.(*ast.Ident); ok {
		pkgName = id.Name
	}
	lower := strings.ToLower(pkgName)
	return lower == "pgxpool" || strings.Contains(lower, "pool")
}

func checkDSNCall(fset *token.FileSet, dsnExpr ast.Expr, dsn string, dm *directives.DirectiveMap, maxSafe int32, issues *[]Issue) {
	if !strings.Contains(dsn, "://") && !strings.Contains(dsn, "host=") && !strings.Contains(dsn, "dbname=") {
		return
	}
	if fset != nil && dm != nil && (dm.IsIgnored(fset, dsnExpr.Pos(), RuleCode) || dm.IsIgnored(fset, dsnExpr.Pos(), RuleCode+".MAX-CONNS")) {
		return
	}

	eval := EvaluateDSN(dsn, maxSafe)
	if !eval.Valid {
		*issues = append(*issues, Issue{
			Pos:     dsnExpr.Pos(),
			Message: eval.Message,
		})
	}
}

func checkNewWithConfigCall(fset *token.FileSet, call *ast.CallExpr, cfgExpr ast.Expr, file *ast.File, dm *directives.DirectiveMap, maxSafe int32, issues *[]Issue) {
	if fset != nil && dm != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".MAX-CONNS") || dm.IsIgnored(fset, cfgExpr.Pos(), RuleCode)) {
		return
	}

	res := TrackConfig(cfgExpr, file, maxSafe)
	if !res.Valid {
		reportPos := call.Pos()
		if fset != nil && dm != nil {
			if dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".MAX-CONNS") {
				return
			}
			if dm.IsIgnored(fset, cfgExpr.Pos(), RuleCode) || dm.IsIgnored(fset, cfgExpr.Pos(), RuleCode+".MAX-CONNS") {
				return
			}
			if res.ReportNode != nil && (dm.IsIgnored(fset, res.ReportNode.Pos(), RuleCode) || dm.IsIgnored(fset, res.ReportNode.Pos(), RuleCode+".MAX-CONNS")) {
				return
			}
		}
		*issues = append(*issues, Issue{
			Pos:     reportPos,
			Message: res.Message,
		})
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	maxSafe := int32(cfg.GetInt(RuleCode, "max_safe_conns", int(DefaultMaxSafeConns)))
	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	for _, file := range pass.Files {
		issues := InspectFile(pass, pass.Fset, file, dm, maxSafe)
		for _, iss := range issues {
			pass.Reportf(iss.Pos, "[%s] %s", RuleCode, iss.Message)
		}
	}

	return nil, nil
}
