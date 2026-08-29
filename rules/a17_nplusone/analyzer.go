// Package a17_nplusone detects and eliminates N+1 database query patterns inside loops
// in favor of set-based (ANY($1)) or batch operations.
package a17_nplusone

import (
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// RuleCode is the official identifier for ARGUS-A17.
const RuleCode = "ARGUS-A17"

// Analyzer defines the analysis.Analyzer for rule ARGUS-A17.
var Analyzer = &analysis.Analyzer{
	Name: "a17",
	Doc:  "Detect and eliminate N+1 database query patterns inside loops in favor of set-based or batch operations",
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

		detector := NewHelperQueryDetector(pass, file)
		issues := WalkLoops(pass, pass.Fset, file, dm, detector)
		for _, issue := range issues {
			pass.Reportf(issue.Pos, "%s", issue.Message)
		}
	}

	return nil, nil
}
