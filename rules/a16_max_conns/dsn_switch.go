// Package a16_max_conns tracks switch control flow and branch fallthrough for DSN analysis.
package a16_max_conns

import (
	"go/ast"
	"go/token"
)

func evalDSNSwitchFlow(file *ast.File, s *ast.SwitchStmt, targetPos token.Pos, varName string, targetObj *ast.Object, inSet []string, depth int) ([]string, bool) {
	if s.Init != nil {
		var reached bool
		inSet, reached = evalDSNStmtFlow(file, s.Init, targetPos, varName, targetObj, inSet, depth)
		if reached {
			return inSet, true
		}
	}
	if s.Body == nil {
		return inSet, false
	}
	if targetPos != token.NoPos && s.Body.Pos() <= targetPos && targetPos <= s.Body.End() {
		hasFallthrough := false
		fallthroughSet := []string(nil)
		for _, stmt := range s.Body.List {
			cc, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			caseIn := inSet
			if hasFallthrough {
				caseIn = deduplicateStrings(append(append([]string{}, inSet...), fallthroughSet...))
			}
			if cc.Pos() <= targetPos && targetPos <= cc.End() {
				return evalDSNStmtList(file, cc.Body, targetPos, varName, targetObj, caseIn, depth)
			}
			outSet, _ := evalDSNStmtList(file, cc.Body, targetPos, varName, targetObj, caseIn, depth)
			if caseEndsWithFallthrough(cc.Body) {
				hasFallthrough = true
				fallthroughSet = outSet
			} else {
				hasFallthrough = false
				fallthroughSet = nil
			}
		}
		return inSet, true
	}

	var branchSets [][]string
	hasDefault := false
	allTerm := true
	hasFallthrough := false
	fallthroughSet := []string(nil)

	for _, stmt := range s.Body.List {
		cc, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		if cc.List == nil {
			hasDefault = true
		}
		caseIn := inSet
		if hasFallthrough {
			caseIn = deduplicateStrings(append(append([]string{}, inSet...), fallthroughSet...))
		}
		caseSet, _ := evalDSNStmtList(file, cc.Body, targetPos, varName, targetObj, caseIn, depth)
		if caseEndsWithFallthrough(cc.Body) {
			hasFallthrough = true
			fallthroughSet = caseSet
			continue
		}
		hasFallthrough = false
		fallthroughSet = nil
		if !isCaseTerminating(cc.Body) {
			allTerm = false
			branchSets = append(branchSets, caseSet)
		}
	}
	if !hasDefault && !allTerm {
		branchSets = append(branchSets, inSet)
	}
	if len(branchSets) == 0 {
		return inSet, false
	}
	var joined []string
	for _, bs := range branchSets {
		joined = append(joined, bs...)
	}
	return deduplicateStrings(joined), false
}

func isCaseTerminating(body []ast.Stmt) bool {
	return len(body) > 0 && isTerminating(body[len(body)-1])
}

func exprListContainsPos(list []ast.Expr, pos token.Pos) bool {
	for _, e := range list {
		if e != nil && e.Pos() <= pos && pos <= e.End() {
			return true
		}
	}
	return false
}
