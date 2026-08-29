// Package a23_tx_timeout enforces explicit transaction_timeout GUC parameter on pgxpool
// configuration for PostgreSQL 17/18+ targets to prevent XID horizon freezing and dead tuple bloat.
package a23_tx_timeout

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A23.
const RuleCode = "ARGUS-A23"

// Analyzer defines the analysis.Analyzer for ARGUS-A23.
var Analyzer = &analysis.Analyzer{
	Name: "argus_a23_transaction_timeout_config",
	Doc:  "Enforce explicit transaction_timeout GUC parameter on pgxpool configuration for PostgreSQL 17/18+ targets (CWE-400)",
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

		InspectFile(file, pass.Fset, dm, func(pos token.Pos, format string, args ...any) {
			pass.Reportf(pos, format, args...)
		})
	}

	return nil, nil
}

// InspectFile walks an AST file and reports violations of ARGUS-A23.
func InspectFile(file *ast.File, fset *token.FileSet, dm *directives.DirectiveMap, report func(pos token.Pos, format string, args ...any)) {
	hasFileTxConfig, isFileZero := InspectFileForTxTimeout(file)

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		// Check composite literals initializing RuntimeParams map
		if compLit, ok := n.(*ast.CompositeLit); ok {
			if mapType, ok := compLit.Type.(*ast.MapType); ok {
				if isStringMap(mapType) && hasOtherTimeouts(compLit) {
					hasTx, isZero := CheckRuntimeParamsMap(compLit)
					if isZero {
						if !isIgnored(dm, fset, compLit.Pos()) {
							report(compLit.Pos(), "[%s] 'transaction_timeout' set to 0; unbounded transaction duration risks XID horizon freezing (CWE-400)", RuleCode)
						}
					} else if !hasTx {
						if !isIgnored(dm, fset, compLit.Pos()) {
							report(compLit.Pos(), "[%s] pgxpool RuntimeParams missing 'transaction_timeout' GUC for PostgreSQL 17/18+ targets; recommend 30000ms-60000ms (CWE-400)", RuleCode)
						}
					}
				}
			}
		}

		// Check pgxpool.New(ctx, dsn) or pgxpool.NewWithConfig(ctx, cfg) calls
		if call, ok := n.(*ast.CallExpr); ok {
			methodName := callsite.GetCallMethodName(call.Fun)
			switch methodName {
			case "New", "pgxpool.New":
				if len(call.Args) >= 2 {
					if dsnStr, ok := callsite.ExtractQueryString(call); ok {
						hasTx, isZero := CheckDSN(dsnStr)
						if isZero {
							if !isIgnored(dm, fset, call.Pos()) {
								report(call.Pos(), "[%s] DSN 'transaction_timeout' set to 0; unbounded transaction duration risks XID horizon freezing (CWE-400)", RuleCode)
							}
						} else if !hasTx {
							if !isIgnored(dm, fset, call.Pos()) {
								report(call.Pos(), "[%s] pgxpool DSN missing 'transaction_timeout' parameter for PostgreSQL 17/18+ targets; recommend 30000ms-60000ms (CWE-400)", RuleCode)
							}
						}
					}
				}
			case "NewWithConfig", "pgxpool.NewWithConfig":
				if !hasFileTxConfig {
					if !isIgnored(dm, fset, call.Pos()) {
						report(call.Pos(), "[%s] pgxpool initialization missing 'transaction_timeout' GUC configuration for PostgreSQL 17/18+ targets (CWE-400)", RuleCode)
					}
				} else if isFileZero {
					if !isIgnored(dm, fset, call.Pos()) {
						report(call.Pos(), "[%s] 'transaction_timeout' set to 0; unbounded transaction duration risks XID horizon freezing (CWE-400)", RuleCode)
					}
				}
			}
		}

		return true
	})
}

func isStringMap(mapType *ast.MapType) bool {
	if keyIdent, ok := mapType.Key.(*ast.Ident); ok && keyIdent.Name == "string" {
		if valIdent, ok := mapType.Value.(*ast.Ident); ok && valIdent.Name == "string" {
			return true
		}
	}
	return false
}

func hasOtherTimeouts(compLit *ast.CompositeLit) bool {
	for _, elt := range compLit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			key := extractStringLit(kv.Key)
			if key == "statement_timeout" || key == "lock_timeout" || key == "idle_in_transaction_session_timeout" {
				return true
			}
		}
	}
	return false
}

func isIgnored(dm *directives.DirectiveMap, fset *token.FileSet, pos token.Pos) bool {
	if dm == nil {
		return false
	}
	return dm.IsIgnored(fset, pos, RuleCode) || dm.IsIgnored(fset, pos, RuleCode+".TX-TIMEOUT")
}
