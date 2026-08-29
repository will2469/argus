// Package a18_rows_err enforces mandatory rows.Err() checks immediately after
// database cursor loops (for rows.Next()) to prevent silent dataset truncation.
package a18_rows_err

import (
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A18.
const RuleCode = "ARGUS-A18"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A18.
var Analyzer = &analysis.Analyzer{
	Name: "a18",
	Doc:  "Enforce mandatory rows.Err() check immediately after for rows.Next() loop to prevent silent dataset truncation",
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

		issues := InspectFile(pass, pass.Fset, file, dm)
		for _, issue := range issues {
			pass.Reportf(issue.Pos, "%s", issue.Message)
		}
	}

	return nil, nil
}
