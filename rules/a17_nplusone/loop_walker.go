package a17_nplusone

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/directives"
)

// LoopIssue describes an N+1 query issue found in a loop construct.
type LoopIssue struct {
	Pos     token.Pos
	End     token.Pos
	Message string
}

// WalkLoops traverses loops in a file, tracking depth and identifying N+1 patterns.
func WalkLoops(pass *analysis.Pass, fset *token.FileSet, file *ast.File, dm *directives.DirectiveMap, detector *HelperQueryDetector) []LoopIssue {
	if fset == nil && pass != nil {
		fset = pass.Fset
	}
	var issues []LoopIssue
	walkLoopNode(pass, fset, file, 0, dm, detector, &issues)
	return issues
}

func walkLoopNode(pass *analysis.Pass, fset *token.FileSet, node ast.Node, currentDepth int, dm *directives.DirectiveMap, detector *HelperQueryDetector, issues *[]LoopIssue) {
	if node == nil {
		return
	}

	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil || n == node {
			return true
		}

		switch x := n.(type) {
		case *ast.RangeStmt:
			// Ignore range over integer literals (e.g., `for range 3` retry loops)
			if lit, ok := x.X.(*ast.BasicLit); ok && lit.Kind == token.INT {
				return true
			}
			depth := currentDepth + 1
			checkLoopBody(pass, fset, x, x.Body, depth, dm, detector, issues)
			walkLoopNode(pass, fset, x.Body, depth, dm, detector, issues)
			return false

		case *ast.ForStmt:
			depth := currentDepth + 1
			checkLoopBody(pass, fset, x, x.Body, depth, dm, detector, issues)
			walkLoopNode(pass, fset, x.Body, depth, dm, detector, issues)
			return false
		}

		return true
	})
}

func checkLoopBody(pass *analysis.Pass, fset *token.FileSet, loopStmt ast.Stmt, body *ast.BlockStmt, depth int, dm *directives.DirectiveMap, detector *HelperQueryDetector, issues *[]LoopIssue) {
	if body == nil {
		return
	}

	// 1. Check if the loop header has an ignore directive
	if dm != nil && fset != nil && (dm.IsIgnored(fset, loopStmt.Pos(), RuleCode) || dm.IsIgnored(fset, loopStmt.Pos(), RuleCode+".NPLUSONE")) {
		return
	}

	// 2. Search for direct DB queries or helper calls in immediate body (not in nested loops)
	foundCall, helperName := findQueryInImmediateBody(pass, fset, body, dm, detector)
	if foundCall == nil {
		return
	}

	var msg string
	if helperName != "" {
		msg = fmt.Sprintf("[%s] N+1 query pattern detected: helper function %q executes database query inside loop; use batching or set-based query instead", RuleCode, helperName)
	} else if depth <= 1 {
		msg = fmt.Sprintf("[%s] N+1 query pattern detected (loop + Query); use set-based query (ANY($1)) or batch operations instead of querying inside loop", RuleCode)
	} else {
		msg = fmt.Sprintf("[%s] nested N+1 query pattern detected (loop depth %d + Query); use set-based query (ANY($1)) or batch operations instead of querying inside loop", RuleCode, depth)
	}

	*issues = append(*issues, LoopIssue{
		Pos:     loopStmt.Pos(),
		End:     loopStmt.End(),
		Message: msg,
	})
}

func findQueryInImmediateBody(pass *analysis.Pass, fset *token.FileSet, body *ast.BlockStmt, dm *directives.DirectiveMap, detector *HelperQueryDetector) (*ast.CallExpr, string) {
	var foundCall *ast.CallExpr
	var foundHelper string

	ast.Inspect(body, func(n ast.Node) bool {
		if foundCall != nil {
			return false
		}

		// Do not inspect inner loops; inner loops are walked recursively with their own depth
		if _, ok := n.(*ast.ForStmt); ok && n != body {
			return false
		}
		if _, ok := n.(*ast.RangeStmt); ok && n != body {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if dm != nil && fset != nil && (dm.IsIgnored(fset, call.Pos(), RuleCode) || dm.IsIgnored(fset, call.Pos(), RuleCode+".NPLUSONE")) {
			return true
		}

		if IsDBQueryCall(pass, call) {
			foundCall = call
			return false
		}

		if detector != nil {
			if isHelper, name := detector.CheckHelperCall(pass, call); isHelper {
				foundCall = call
				foundHelper = name
				return false
			}
		}

		return true
	})

	return foundCall, foundHelper
}
