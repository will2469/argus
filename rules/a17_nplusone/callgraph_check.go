package a17_nplusone

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// HelperQueryDetector caches and detects functions that execute database queries across a package.
type HelperQueryDetector struct {
	// Primary type-safe mappings
	queryByObj map[types.Object]bool
	declByObj  map[types.Object]*ast.FuncDecl
	objToName  map[types.Object]string

	// String fallback mappings (for nil pass.TypesInfo or unresolvable objects)
	queryByStr map[string]bool
	declByStr  map[string]*ast.FuncDecl

	// Backward-compatible aliases for external tests
	funcHasQuery map[string]bool
	funcDecls    map[string]*ast.FuncDecl
}

// NewHelperQueryDetector builds a package-wide or file-wide call graph index.
func NewHelperQueryDetector(pass *analysis.Pass, files ...*ast.File) *HelperQueryDetector {
	detector := &HelperQueryDetector{
		queryByObj: make(map[types.Object]bool),
		declByObj:  make(map[types.Object]*ast.FuncDecl),
		objToName:  make(map[types.Object]string),
		queryByStr: make(map[string]bool),
		declByStr:  make(map[string]*ast.FuncDecl),
	}
	detector.funcHasQuery = detector.queryByStr
	detector.funcDecls = detector.declByStr

	targetFiles := files
	if len(targetFiles) == 0 && pass != nil {
		targetFiles = pass.Files
	}

	// 1. Index all function and method declarations
	for _, file := range targetFiles {
		if file == nil {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Body == nil {
				continue
			}

			// Index by types.Object when available
			if pass != nil && pass.TypesInfo != nil {
				if obj := pass.TypesInfo.Defs[fn.Name]; obj != nil {
					detector.declByObj[obj] = fn
					detector.objToName[obj] = formatObjName(obj, fn)
				}
			}

			// Index by string key fallback
			strKey := getFuncDeclKey(fn)
			if strKey != "" {
				detector.declByStr[strKey] = fn
			}
		}
	}

	// 2. Mark functions that execute DB queries directly
	for obj, fn := range detector.declByObj {
		if containsDirectDBQuery(pass, fn.Body) {
			detector.queryByObj[obj] = true
			if strKey := getFuncDeclKey(fn); strKey != "" {
				detector.queryByStr[strKey] = true
			}
		}
	}
	for key, fn := range detector.declByStr {
		if containsDirectDBQuery(pass, fn.Body) {
			detector.queryByStr[key] = true
		}
	}

	// 3. Fixed-point propagation: propagate query execution transitively
	changed := true
	for changed {
		changed = false

		// Propagate via types.Object
		if pass != nil && pass.TypesInfo != nil {
			for obj, fn := range detector.declByObj {
				if detector.queryByObj[obj] {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if detector.queryByObj[obj] {
						return false
					}
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					if calledObj := resolveCallObj(pass.TypesInfo, call); calledObj != nil {
						if detector.queryByObj[calledObj] {
							detector.queryByObj[obj] = true
							if strKey := getFuncDeclKey(fn); strKey != "" {
								detector.queryByStr[strKey] = true
							}
							changed = true
							return false
						}
					}
					return true
				})
			}
		}

		// Propagate via string key fallback
		for key, fn := range detector.declByStr {
			if detector.queryByStr[key] {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if detector.queryByStr[key] {
					return false
				}
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				calledKey := resolveCallStrKey(pass, call)
				if calledKey != "" && detector.queryByStr[calledKey] {
					detector.queryByStr[key] = true
					changed = true
					return false
				}
				return true
			})
		}
	}

	return detector
}

// CheckHelperCall checks if a call expression invokes a helper function that executes a DB query.
func (d *HelperQueryDetector) CheckHelperCall(pass *analysis.Pass, call *ast.CallExpr) (bool, string) {
	if d == nil || call == nil {
		return false, ""
	}

	// 1. Primary: resolve types.Object
	if pass != nil && pass.TypesInfo != nil {
		if obj := resolveCallObj(pass.TypesInfo, call); obj != nil {
			if d.queryByObj[obj] {
				name := d.objToName[obj]
				if name == "" {
					name = obj.Name()
				}
				return true, name
			}
			// If obj is known to be declared in this package and has no query, do not fall back to string keys!
			if _, declared := d.declByObj[obj]; declared {
				return false, ""
			}
		}
	}

	// 2. Fallback: string key resolution
	key := resolveCallStrKey(pass, call)
	if key != "" && d.queryByStr[key] {
		return true, key
	}

	return false, ""
}

func containsDirectDBQuery(pass *analysis.Pass, body *ast.BlockStmt) bool {
	hasQuery := false
	ast.Inspect(body, func(n ast.Node) bool {
		if hasQuery {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if IsDBQueryCall(pass, call) {
			hasQuery = true
			return false
		}
		return true
	})
	return hasQuery
}
