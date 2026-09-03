package a26_like_sanitize

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"gopkg.in/yaml.v3"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// TestSanitizerMatrix_TypedAndUnresolved rigorously evaluates the 5-case sanitizer identity matrix:
// 1. typed path: Evil.SanitizeLikePattern()                 -> MUST FAIL
// 2. typed path: trusted.SanitizeLikePattern()              -> MUST PASS
// 3. unresolved path: evil.SanitizeLikePattern()           -> MUST FAIL
// 4. configured exact symbol: trusted.SanitizeLikePattern() -> MUST PASS
// 5. same method, wrong package: evil.SanitizeLikePattern() -> MUST FAIL
func TestSanitizerMatrix_TypedAndUnresolved(t *testing.T) {
	// Parse YAML configuration mimicking .argus.yaml
	yamlData := []byte(`
version: "1"
rules:
  ARGUS-A26:
    enabled: true
    sanitizers:
      - "dummy/sec/trusted.SanitizeLikePattern"
`)
	appCfg := config.DefaultConfig()
	if err := yaml.Unmarshal(yamlData, appCfg); err != nil {
		t.Fatalf("failed to unmarshal .argus.yaml: %v", err)
	}

	// ---------------------------------------------------------------------
	// Part 1: Typed Path Verification (pass.TypesInfo available)
	// ---------------------------------------------------------------------
	srcLocal := `package test

type Evil struct{}
func (Evil) SanitizeLikePattern(s string) string { return s }

func Run(e Evil, input string) {
	_ = e.SanitizeLikePattern(input)
}
`
	fset := token.NewFileSet()
	fLocal, err := parser.ParseFile(fset, "local.go", srcLocal, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	typesInfo := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	conf := types.Config{Error: func(err error) {}}
	pkg, _ := conf.Check("test", fset, []*ast.File{fLocal}, typesInfo)
	pass := &analysis.Pass{
		Fset:      fset,
		Files:     []*ast.File{fLocal},
		Pkg:       pkg,
		TypesInfo: typesInfo,
		ResultOf: map[*analysis.Analyzer]any{
			config.Analyzer: appCfg,
		},
	}

	regTyped := NewSanitizerRegistry(pass, []*ast.File{fLocal}, appCfg, nil)

	var evilCall *ast.CallExpr
	ast.Inspect(fLocal, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "SanitizeLikePattern" {
				evilCall = c
			}
		}
		return true
	})
	if evilCall == nil {
		t.Fatal("failed to locate evilCall")
	}

	// Matrix 1: typed path Evil.SanitizeLikePattern() -> MUST FAIL
	if regTyped.IsSanitizerCall(pass, evilCall) {
		t.Errorf("MATRIX 1 FAILED: typed Evil.SanitizeLikePattern() must be REJECTED")
	}

	// Matrix 2: typed path trusted.SanitizeLikePattern() -> MUST PASS
	trustedPkg := types.NewPackage("dummy/sec/trusted", "trusted")
	trustedFunc := types.NewFunc(token.NoPos, trustedPkg, "SanitizeLikePattern", types.NewSignatureType(nil, nil, nil, nil, nil, false))
	trustedCallIdent := &ast.Ident{Name: "SanitizeLikePattern"}
	typesInfo.Uses[trustedCallIdent] = trustedFunc
	trustedCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "trusted"},
			Sel: trustedCallIdent,
		},
	}
	if !regTyped.IsSanitizerCall(pass, trustedCall) {
		t.Errorf("MATRIX 2 FAILED: typed trusted.SanitizeLikePattern() must PASS")
	}

	// ---------------------------------------------------------------------
	// Part 2: Unresolved / Standalone Fallback Path (pass == nil)
	// ---------------------------------------------------------------------
	srcUnresolved := `package test

import (
	"dummy/sec/trusted"
	evilpkg "dummy/sec/evil"
)

type Evil struct{}
func (Evil) SanitizeLikePattern(s string) string { return s }

func Run(e Evil, input string) {
	_ = e.SanitizeLikePattern(input)
	_ = trusted.SanitizeLikePattern(input)
	_ = evilpkg.SanitizeLikePattern(input)
}
`
	fsetUnres := token.NewFileSet()
	fUnres, err := parser.ParseFile(fsetUnres, "unres.go", srcUnresolved, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	regUnres := NewSanitizerRegistry(nil, []*ast.File{fUnres}, appCfg, nil)

	var calls []*ast.CallExpr
	ast.Inspect(fUnres, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if _, ok := c.Fun.(*ast.SelectorExpr); ok {
				calls = append(calls, c)
			}
		}
		return true
	})

	if len(calls) < 3 {
		t.Fatalf("expected at least 3 calls, got %d", len(calls))
	}

	// Matrix 3: unresolved path evil.SanitizeLikePattern() -> MUST FAIL
	if regUnres.IsSanitizerCall(nil, calls[0]) {
		t.Errorf("MATRIX 3 FAILED: unresolved evil.SanitizeLikePattern() must FAIL")
	}

	// Matrix 4: configured exact symbol trusted.SanitizeLikePattern() -> MUST PASS
	if !regUnres.IsSanitizerCall(nil, calls[1]) {
		t.Errorf("MATRIX 4 FAILED: unresolved configured trusted.SanitizeLikePattern() must PASS")
	}

	// Matrix 5: same method, wrong package evilpkg.SanitizeLikePattern() -> MUST FAIL
	if regUnres.IsSanitizerCall(nil, calls[2]) {
		t.Errorf("MATRIX 5 FAILED: unresolved same method wrong package evilpkg.SanitizeLikePattern() must FAIL")
	}
}

// TestArgusYaml_SanitizersWiring_EndToEnd verifies that .argus.yaml configures InspectFile end-to-end.
func TestArgusYaml_SanitizersWiring_EndToEnd(t *testing.T) {
	yamlCfg := `
version: "1"
rules:
  ARGUS-A26:
    enabled: true
    sanitizers:
      - "dummy/sec/trusted.SanitizeLikePattern"
`
	appCfg := config.DefaultConfig()
	if err := yaml.Unmarshal([]byte(yamlCfg), appCfg); err != nil {
		t.Fatalf("failed to unmarshal .argus.yaml: %v", err)
	}

	src := `package test

import (
	"context"
	"dummy/sec/trusted"
	evilpkg "dummy/sec/evil"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any)
}

func SafeSearch(ctx context.Context, db DB, input string) {
	// Compliant: trusted.SanitizeLikePattern is whitelisted in .argus.yaml
	db.Query(ctx, "SELECT id FROM users WHERE name LIKE $1", trusted.SanitizeLikePattern(input))
}

func EvilSearch(ctx context.Context, db DB, input string) {
	// Violation: evilpkg.SanitizeLikePattern is NOT in .argus.yaml
	db.Query(ctx, "SELECT id FROM users WHERE name LIKE $1", evilpkg.SanitizeLikePattern(input))
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "query.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	dm := directives.NewDirectiveMap()

	// 1. Driver execution mode: appCfg wired through pass.ResultOf[config.Analyzer]
	pass := &analysis.Pass{
		Fset:  fset,
		Files: []*ast.File{f},
		ResultOf: map[*analysis.Analyzer]any{
			config.Analyzer: appCfg,
		},
	}
	issues := InspectFile(pass, fset, f, dm)

	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue for EvilSearch, got %d", len(issues))
	}
	if !strings.Contains(issues[0].Message, "unsanitized wildcard parameter") {
		t.Errorf("unexpected issue message: %s", issues[0].Message)
	}

	// 2. Standalone runner mode: appCfg wired via InspectFileWithConfig
	standaloneIssues := InspectFileWithConfig(nil, fset, f, dm, appCfg)
	if len(standaloneIssues) != 1 {
		t.Fatalf("expected standalone to produce exactly 1 issue, got %d", len(standaloneIssues))
	}
	_ = context.Background // keep context imported if needed
}
