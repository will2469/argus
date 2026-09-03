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
	pkgImports   map[string]string // alias/name -> full import path
}

// NewSanitizerRegistry builds a registry from AST files, config, and directives.
func NewSanitizerRegistry(pass *analysis.Pass, files []*ast.File, cfg *config.Config, dm *directives.DirectiveMap) *SanitizerRegistry {
	reg := &SanitizerRegistry{
		trustedObj:   make(map[types.Object]bool),
		trustedNames: make(map[string]bool),
		configured:   make(map[string]bool),
		pkgImports:   make(map[string]string),
	}

	// 1. User-configured sanitizers from .argus.yaml
	if cfg != nil {
		for _, s := range cfg.GetStringSlice(RuleCode, "sanitizers", nil) {
			norm := strings.TrimSpace(s)
			if norm != "" {
				reg.configured[norm] = true
			}
		}
	}

	// 2. Index imports across all provided files for unresolved standalone alias resolution
	for _, file := range files {
		if file == nil {
			continue
		}
		for _, imp := range file.Imports {
			if imp == nil || imp.Path == nil {
				continue
			}
			importPath := strings.Trim(imp.Path.Value, `"`)
			if imp.Name != nil && imp.Name.Name != "" {
				if imp.Name.Name != "_" && imp.Name.Name != "." {
					reg.pkgImports[imp.Name.Name] = importPath
				}
			} else {
				parts := strings.Split(importPath, "/")
				last := parts[len(parts)-1]
				if last != "" {
					reg.pkgImports[last] = importPath
				}
			}
		}
	}

	// 3. Index local package functions across all provided files
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

	// 4. Mark functions with explicit // argus:trusted-sanitizer <reason> directive
	for _, fn := range funcDecls {
		if hasTrustedSanitizerDirective(fn.Doc) {
			reg.markTrustedFunc(pass, fn)
		}
	}

	// 5. Statically verify function bodies for explicit wildcard replacements (%, _)
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
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		typeName := extractRecvTypeName(fn.Recv.List[0].Type)
		if typeName != "" {
			r.trustedNames[typeName+"."+fn.Name.Name] = true
		}
	} else {
		// Only top-level package functions without receiver can be trusted by bare function name
		r.trustedNames[fn.Name.Name] = true
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

	// 1. Types-based resolution: authoritative exact package path and symbol
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
				if fn, ok := obj.(*types.Func); ok {
					recv := fn.Type().(*types.Signature).Recv()
					if recv != nil {
						recvTypeName := extractTypeNameFromType(recv.Type())
						if recvTypeName != "" && r.configured[pkg.Path()+"."+recvTypeName+"."+obj.Name()] {
							return true
						}
					}
				}
			}
			// Strict AST Determinism: When type resolution succeeds, never fall through to lexical heuristics!
			return false
		}
	}

	// 2. Unresolved / standalone fallback: strictly scoped by imports and verified local decls
	switch e := call.Fun.(type) {
	case *ast.Ident:
		// Bare function call (e.g. SanitizeLike(input)) — only trusted if local verified function
		if r.trustedNames[e.Name] {
			return true
		}
	case *ast.SelectorExpr:
		methodName := e.Sel.Name
		if xId, ok := e.X.(*ast.Ident); ok {
			// Case A: Imported package call (e.g. trustedpkg.SanitizeLikePattern or mypkg.Sanitize)
			if importPath, isImport := r.pkgImports[xId.Name]; isImport {
				fullSymbol := importPath + "." + methodName
				if r.configured[fullSymbol] {
					return true
				}
				return false
			}

			// Case B: Local receiver or qualified type (e.g. VerifiedEscaper.SanitizeLikePattern)
			qualified := xId.Name + "." + methodName
			if r.trustedNames[qualified] {
				return true
			}
		}
	}

	return false
}
