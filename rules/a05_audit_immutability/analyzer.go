// Package a05_audit_immutability enforces that audit log tables remain append-only
// by forbidding UPDATE, DELETE, TRUNCATE, MERGE, and DROP operations.
package a05_audit_immutability

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A05"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A05.
var Analyzer = &analysis.Analyzer{
	Name: "a05",
	Doc:  "Enforces strict append-only immutability on audit log tables (prohibiting UPDATE, DELETE, TRUNCATE, MERGE, DROP)",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A05.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for forbidden mutations on audit tables.
// Can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap, auditTables map[string]bool) []Issue {
	if file == nil {
		return nil
	}
	if fset == nil && pass != nil {
		fset = pass.Fset
	}
	if auditTables == nil {
		auditTables = map[string]bool{"audit_logs": true, "security_events": true}
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
		inspectFunction(pass, fset, file, fn, dm, auditTables, &issues)
	}

	return issues
}

func inspectFunction(pass *analysis.Pass, fset *token.FileSet, file *ast.File, fn *ast.FuncDecl, dm *directives.DirectiveMap, auditTables map[string]bool, issues *[]Issue) {
	tracker := analyzeFunctionFlow(pass, file, fn)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !callsite.IsDBQueryMethod(sel.Sel.Name) {
			return true
		}

		// Filter non-db receivers
		if id, ok := sel.X.(*ast.Ident); ok {
			switch strings.ToLower(id.Name) {
			case "search", "client", "http", "logger", "cmd", "runner", "cache", "ceremony":
				return true
			}
		} else if selRecv, ok := sel.X.(*ast.SelectorExpr); ok {
			switch strings.ToLower(selRecv.Sel.Name) {
			case "search", "client", "http", "logger", "cmd", "runner", "cache", "ceremony":
				return true
			}
		}

		if dm != nil && fset != nil && dm.IsIgnored(fset, call.Pos(), RuleCode) {
			return true
		}

		queries := tracker.ResolveCallQueries(call)
		// nil means no SQL argument found, skip
		if queries == nil {
			return true
		}
		// empty slice []string{} means Unknown provenance (not provably safe) - fail-closed
		if len(queries) == 0 {
			// Query and QueryRow are read operations; forensic SELECT on audit logs is explicitly permitted.
			if sel.Sel.Name == "Query" || sel.Sel.Name == "QueryRow" {
				return true
			}
			// If already vetted and explicitly suppressed for dynamic SQL (A01), do not double-flag under A05
			if dm != nil && fset != nil && (dm.IsIgnored(fset, call.Pos(), "ARGUS-A01") || dm.IsIgnored(fset, call.Pos(), "A01")) {
				return true
			}

			// For security-sensitive audit table operations, Unknown is not provably safe
			// We flag this to enforce explicit whitelisting or provenance
			*issues = append(*issues, Issue{
				Pos:     call.Pos(),
				Message: "query provenance is unknown (not provably safe); audit table operations require explicit static provenance",
			})
			return true
		}

		for _, query := range queries {
			if strings.TrimSpace(query) == "" {
				continue
			}

			op, violatedTable := CheckSQLTampering(query, auditTables)
			if op != "" {
				*issues = append(*issues, Issue{
					Pos:     call.Pos(),
					Message: fmt.Sprintf("forbidden %s on audit table %q; audit trails must be strictly append-only", op, violatedTable),
				})
				break
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

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)

	auditTablesMap := make(map[string]bool)
	auditTables := cfg.GetStringSlice(RuleCode, "audit_tables", []string{"audit_logs", "security_events"})
	for _, t := range auditTables {
		auditTablesMap[strings.ToLower(strings.TrimSpace(t))] = true
	}

	// 1. Inspect Go source files
	for _, file := range pass.Files {
		issues := InspectFile(pass, pass.Fset, file, dm, auditTablesMap)
		for _, iss := range issues {
			pass.Reportf(iss.Pos, "[%s] %s", RuleCode, iss.Message)
		}
	}

	// 2. Inspect migrations if current package directory hosts migrations
	if len(pass.Files) > 0 {
		pkgDir := filepath.Dir(pass.Fset.Position(pass.Files[0].Pos()).Filename)
		migDir := cfg.FindMatchingMigrationDir(pkgDir)
		if migDir != "" {
			checkDown := cfg.GetBool(RuleCode, "check_down_migrations", false)
			issues := InspectMigrationDir(migDir, dm, auditTablesMap, checkDown)
			for _, is := range issues {
				pass.Reportf(pass.Files[0].Pos(), "[%s] %s:%d: %s", RuleCode, filepath.Base(is.Filename), is.Line, is.Message)
			}
		}
	}

	return nil, nil
}
