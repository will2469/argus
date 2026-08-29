// Package a16_max_conns enforces explicit, bounded MaxConns configuration on pgxpool
// to prevent Linux kernel process thrashing, memory exhaustion, and connection starvation.
package a16_max_conns

import (
	"go/ast"
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

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	maxSafe := int32(cfg.GetInt(RuleCode, "max_safe_conns", int(DefaultMaxSafeConns)))
	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	for _, file := range pass.Files {
		pos := pass.Fset.Position(file.Package)
		if strings.HasSuffix(pos.Filename, "_test.go") {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			inspectCall(pass, call, file, dm, maxSafe)
			return true
		})
	}

	return nil, nil
}

func inspectCall(pass *analysis.Pass, call *ast.CallExpr, file *ast.File, dm *directives.DirectiveMap, maxSafe int32) {
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
			dsnStr, ok := callsite.ExtractQueryString(call)
			if ok {
				checkDSNCall(pass, call.Args[dsnArgIdx], dsnStr, dm, maxSafe)
			}
		}
	case "NewWithConfig", "pgxpool.NewWithConfig":
		cfgArgIdx := 0
		if len(call.Args) >= 2 {
			cfgArgIdx = 1
		}
		if cfgArgIdx < len(call.Args) {
			checkNewWithConfigCall(pass, call, call.Args[cfgArgIdx], file, dm, maxSafe)
		}
	}
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

func checkDSNCall(pass *analysis.Pass, dsnExpr ast.Expr, dsn string, dm *directives.DirectiveMap, maxSafe int32) {
	if !strings.Contains(dsn, "://") && !strings.Contains(dsn, "host=") && !strings.Contains(dsn, "dbname=") {
		return
	}
	if dm != nil && (dm.IsIgnored(pass.Fset, dsnExpr.Pos(), RuleCode) || dm.IsIgnored(pass.Fset, dsnExpr.Pos(), RuleCode+".MAX-CONNS")) {
		return
	}

	eval := EvaluateDSN(dsn, maxSafe)
	if !eval.Valid {
		pass.Reportf(dsnExpr.Pos(), "[%s] %s", RuleCode, eval.Message)
	}
}

func checkNewWithConfigCall(pass *analysis.Pass, call *ast.CallExpr, cfgExpr ast.Expr, file *ast.File, dm *directives.DirectiveMap, maxSafe int32) {
	if dm != nil && (dm.IsIgnored(pass.Fset, call.Pos(), RuleCode) || dm.IsIgnored(pass.Fset, call.Pos(), RuleCode+".MAX-CONNS") || dm.IsIgnored(pass.Fset, cfgExpr.Pos(), RuleCode)) {
		return
	}

	res := TrackConfig(cfgExpr, file, maxSafe)
	if !res.Valid {
		reportPos := call.Pos()
		if dm != nil {
			if dm.IsIgnored(pass.Fset, call.Pos(), RuleCode) || dm.IsIgnored(pass.Fset, call.Pos(), RuleCode+".MAX-CONNS") {
				return
			}
			if dm.IsIgnored(pass.Fset, cfgExpr.Pos(), RuleCode) || dm.IsIgnored(pass.Fset, cfgExpr.Pos(), RuleCode+".MAX-CONNS") {
				return
			}
			if res.ReportNode != nil && (dm.IsIgnored(pass.Fset, res.ReportNode.Pos(), RuleCode) || dm.IsIgnored(pass.Fset, res.ReportNode.Pos(), RuleCode+".MAX-CONNS")) {
				return
			}
		}
		pass.Reportf(reportPos, "[%s] %s", RuleCode, res.Message)
	}
}
