// Package a20_param_limit inspects AST nodes for unchunked dynamic query generation.
package a20_param_limit

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

var (
	inPlaceholdersRegex = regexp.MustCompile(`(?i)\bIN\s*\(\s*(?:%[sv]|\$|\?)`)
	anyArrayRegex       = regexp.MustCompile(`(?i)=\s*ANY\s*\(\s*(?:\$\d+|\?)\s*\)`)
	valuesKeywordRegex  = regexp.MustCompile(`(?i)\bVALUES\b|\bINSERT\s+INTO\b`)
)

// ExtractPartialQuery resolves string literals, concatenations, or format templates from an expression.
func ExtractPartialQuery(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			val := e.Value
			if len(val) >= 2 && ((val[0] == '`' && val[len(val)-1] == '`') ||
				(val[0] == '"' && val[len(val)-1] == '"')) {
				return val[1 : len(val)-1]
			}
		}
	case *ast.BinaryExpr:
		return ExtractPartialQuery(e.X) + " " + ExtractPartialQuery(e.Y)
	case *ast.CallExpr:
		var parts []string
		for _, a := range e.Args {
			if s := ExtractPartialQuery(a); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	case *ast.Ident:
		if e.Obj != nil {
			if assign, ok := e.Obj.Decl.(*ast.AssignStmt); ok {
				for _, r := range assign.Rhs {
					if s := ExtractPartialQuery(r); s != "" {
						return s
					}
				}
			}
			if spec, ok := e.Obj.Decl.(*ast.ValueSpec); ok {
				for _, v := range spec.Values {
					if s := ExtractPartialQuery(v); s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}

// EvaluateDynamicBatch inspects a call expression and its enclosing function for unbounded dynamic batching.
func EvaluateDynamicBatch(fn ast.Node, call *ast.CallExpr, query string) (DynamicBatchKind, string) {
	if fn == nil || call == nil {
		return BatchKindNone, ""
	}

	// If query was empty from standard extractor, try extracting partial query from arguments
	if strings.TrimSpace(query) == "" {
		for _, arg := range call.Args {
			if s := ExtractPartialQuery(arg); s != "" {
				query = s
				break
			}
		}
	}

	// If the call is already guarded within a chunking loop, it's safe
	if HasEnclosingChunkLoop(fn, call) {
		return BatchKindNone, ""
	}

	// 1. Check for dynamic IN clause construction from slices
	if isDynamicInClause(fn, query) {
		return BatchKindDynamicInClause, "unbounded dynamic IN clause placeholder generation; risk of exceeding 65,535 bind parameter limit; recommend 'WHERE col = ANY($1)' (CWE-400)"
	}

	// 2. Check for dynamic multi-row VALUES batch construction without chunking
	if isDynamicValuesWithoutChunking(fn, query) {
		return BatchKindDynamicValues, "unbounded dynamic multi-row VALUES batch construction without chunking; risk of exceeding 65,535 bind parameter limit; recommend pgx.CopyFrom (CWE-400)"
	}

	return BatchKindNone, ""
}

func isDynamicInClause(fn ast.Node, query string) bool {
	// If the query explicitly uses PostgreSQL array ANY($1), it's safe (uses only 1 parameter)
	if anyArrayRegex.MatchString(query) {
		return false
	}

	// Check if the query template has IN (%s) or dynamic placeholders
	if inPlaceholdersRegex.MatchString(query) {
		return true
	}

	// Check if within the function there is dynamic placeholder generation for IN clauses
	hasDynamicInGen := false
	ast.Inspect(fn, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			funName := getFunctionName(c.Fun)
			if strings.Contains(strings.ToLower(funName), "placeholder") ||
				funName == "strings.Repeat" ||
				funName == "strings.Join" {
				hasDynamicInGen = true
				return false
			}
		}
		return true
	})

	if hasDynamicInGen && strings.Contains(strings.ToUpper(query), " IN ") {
		return true
	}

	return false
}

func isDynamicValuesWithoutChunking(fn ast.Node, query string) bool {
	// Must target INSERT INTO or VALUES
	hasValues := valuesKeywordRegex.MatchString(query)
	if !hasValues {
		return false
	}

	// Check if inside fn there is a loop building query strings or appending args
	hasLoopBuildingValues := false
	ast.Inspect(fn, func(n ast.Node) bool {
		switch loop := n.(type) {
		case *ast.RangeStmt:
			if loopAppendsValuesOrArgs(loop.Body) {
				hasLoopBuildingValues = true
				return false
			}
		case *ast.ForStmt:
			if !IsChunkedLoop(loop) && loopAppendsValuesOrArgs(loop.Body) {
				hasLoopBuildingValues = true
				return false
			}
		}
		return true
	})

	return hasLoopBuildingValues
}

func loopAppendsValuesOrArgs(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	appends := false
	ast.Inspect(body, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			funName := getFunctionName(c.Fun)
			if funName == "append" || strings.Contains(strings.ToLower(funName), "sprintf") {
				appends = true
				return false
			}
		}
		if assign, ok := n.(*ast.AssignStmt); ok && assign.Tok == token.ADD_ASSIGN {
			appends = true
			return false
		}
		return true
	})
	return appends
}

func getFunctionName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		xName := getFunctionName(e.X)
		if xName != "" {
			return xName + "." + e.Sel.Name
		}
		return e.Sel.Name
	}
	return ""
}
