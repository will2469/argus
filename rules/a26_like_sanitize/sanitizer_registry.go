package a26_like_sanitize

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

// SanitizerRegistry maintains verified and trusted LIKE wildcard sanitizers.
type SanitizerRegistry struct {
	trustedObj   map[types.Object]bool
	trustedNames map[string]bool
	configured   map[string]bool
}

// NewSanitizerRegistry builds a registry from AST files, config, and directives.
func NewSanitizerRegistry(pass *analysis.Pass, files []*ast.File, cfg *config.Config, dm *directives.DirectiveMap) *SanitizerRegistry {
	reg := &SanitizerRegistry{
		trustedObj:   make(map[types.Object]bool),
		trustedNames: make(map[string]bool),
		configured:   make(map[string]bool),
	}

	// 1. Built-in canonical packages and user-configured sanitizers
	reg.configured["github.com/will2469/hecate.SanitizeLikePattern"] = true
	reg.configured["github.com/will2469/hecate.FormatLikeContains"] = true
	reg.configured["github.com/will2469/hecate.FormatLikePrefix"] = true
	reg.configured["github.com/will2469/hecate.FormatLikeSuffix"] = true

	if cfg != nil {
		for _, s := range cfg.GetStringSlice(RuleCode, "sanitizers", nil) {
			norm := strings.TrimSpace(s)
			if norm != "" {
				reg.configured[norm] = true
			}
		}
	}

	// 2. Index local package functions across all provided files
	var funcDecls []*ast.FuncDecl
	for _, file := range files {
		if file == nil {
			continue
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name != nil && fn.Body != nil {
				funcDecls = append(funcDecls, fn)
			}
		}
	}

	// 3. Mark functions with explicit // argus:trusted-sanitizer <reason> directive
	for _, fn := range funcDecls {
		if hasTrustedSanitizerDirective(fn.Doc) {
			reg.markTrustedFunc(pass, fn)
		}
	}

	// 4. Statically verify function bodies for explicit wildcard replacements (%, _)
	changed := true
	for changed {
		changed = false
		for _, fn := range funcDecls {
			if reg.isFuncMarkedTrusted(pass, fn) {
				continue
			}
			if isVerifiedEscapingBody(fn.Body, reg, pass) {
				reg.markTrustedFunc(pass, fn)
				changed = true
			}
		}
	}

	return reg
}

func (r *SanitizerRegistry) markTrustedFunc(pass *analysis.Pass, fn *ast.FuncDecl) {
	if fn == nil || fn.Name == nil {
		return
	}
	r.trustedNames[fn.Name.Name] = true
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		typeName := extractRecvTypeName(fn.Recv.List[0].Type)
		if typeName != "" {
			r.trustedNames[typeName+"."+fn.Name.Name] = true
		}
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Defs[fn.Name]; obj != nil {
			r.trustedObj[obj] = true
		}
	}
}

func (r *SanitizerRegistry) isFuncMarkedTrusted(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	if fn == nil || fn.Name == nil {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj := pass.TypesInfo.Defs[fn.Name]; obj != nil && r.trustedObj[obj] {
			return true
		}
	}
	return r.trustedNames[fn.Name.Name]
}

// IsSanitizerCall determines if call invokes a verified or configured sanitizer.
func (r *SanitizerRegistry) IsSanitizerCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	if r == nil || call == nil {
		return false
	}

	// 1. Types-based resolution: exact package path and symbol
	if pass != nil && pass.TypesInfo != nil {
		if obj := resolveCallObj(pass.TypesInfo, call); obj != nil {
			if r.trustedObj[obj] {
				return true
			}
			if pkg := obj.Pkg(); pkg != nil {
				fullPath := pkg.Path() + "." + obj.Name()
				if r.configured[fullPath] {
					return true
				}
			}
			// If declared in this package and not trusted, reject immediately
			if obj.Pkg() == pass.Pkg {
				return false
			}
		}
	}

	// 2. Fallback resolution: inspect call selector / ident
	switch e := call.Fun.(type) {
	case *ast.Ident:
		if r.trustedNames[e.Name] || r.configured[e.Name] {
			return true
		}
	case *ast.SelectorExpr:
		methodName := e.Sel.Name
		if xId, ok := e.X.(*ast.Ident); ok {
			qualified := xId.Name + "." + methodName
			if r.trustedNames[qualified] || r.configured[qualified] {
				return true
			}
			if r.configured[methodName] {
				return true
			}
		}
	}

	return false
}
