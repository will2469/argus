// Package a31_mutation_audit_trail validates that database mutations within transactions
// are accompanied by audit log recording for governance and compliance (SOC 2, ISO 27001).
package a31_mutation_audit_trail

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

const RuleCode = "ARGUS-A31"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A31.
var Analyzer = &analysis.Analyzer{
	Name: "a31",
	Doc:  "Enforces that database mutations inside transactions are accompanied by audit trail logging",
	Run:  run,
	Requires: []*analysis.Analyzer{
		directives.Analyzer,
		config.Analyzer,
	},
}

// Issue describes a detected violation of ARGUS-A31.
type Issue struct {
	Pos     token.Pos
	Message string
}

// InspectFile inspects an AST file for unguarded transaction mutations.
// Can be called with pass == nil in standalone CLI runner mode.
func InspectFile(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap, auditMethods, exemptTables []string) []Issue {
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

	if len(auditMethods) == 0 {
		auditMethods = []string{"SaveTx", "RecordTx", "LogAuditEvent", "Save"}
	}
	if len(exemptTables) == 0 {
		exemptTables = []string{"sessions", "cache", "temporary_tokens"}
	}

	var funcDecls []*ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			funcDecls = append(funcDecls, fn)
		}
	}

	var issues []Issue
	for _, fn := range funcDecls {
		inspectFunction(pass, fset, fn, funcDecls, dm, auditMethods, exemptTables, &issues)
	}

	return issues
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg := pass.ResultOf[config.Analyzer].(*config.Config)
	if !cfg.IsRuleEnabled(RuleCode) {
		return nil, nil
	}

	dm := pass.ResultOf[directives.Analyzer].(*directives.DirectiveMap)
	auditMethods := cfg.GetStringSlice(RuleCode, "audit_methods", []string{"SaveTx", "RecordTx", "LogAuditEvent", "Save"})
	exemptTables := cfg.GetStringSlice(RuleCode, "exempt_tables", []string{"sessions", "cache", "temporary_tokens"})

	for _, file := range pass.Files {
		issues := InspectFile(pass, pass.Fset, file, dm, auditMethods, exemptTables)
		for _, iss := range issues {
			pass.Reportf(iss.Pos, "[%s] %s", RuleCode, iss.Message)
		}
	}

	_ = filepath.Base("")
	return nil, nil
}
