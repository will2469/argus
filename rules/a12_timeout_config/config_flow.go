// Package a12_timeout_config tracks pgxpool.Config initialization and AST assignment flows.
package a12_timeout_config

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// EvalConfigFlow tracks reaching definitions and flow of cfgArg up to call.Pos().
func EvalConfigFlow(pass *analysis.Pass, file *ast.File, cfgArg ast.Expr, call *ast.CallExpr) ConfigStatus {
	var zeroStatus ConfigStatus
	if call == nil || cfgArg == nil {
		return zeroStatus
	}

	if lit, ok := cfgArg.(*ast.CompositeLit); ok {
		return EvalCompositeLit(lit)
	}
	if unary, ok := cfgArg.(*ast.UnaryExpr); ok {
		if lit, ok := unary.X.(*ast.CompositeLit); ok {
			return EvalCompositeLit(lit)
		}
	}

	cfgIdent, ok := cfgArg.(*ast.Ident)
	if !ok {
		return zeroStatus
	}

	enclosingFunc := findEnclosingFunc(file, call.Pos())
	if enclosingFunc == nil || enclosingFunc.Body == nil {
		return zeroStatus
	}

	var targetObj types.Object
	if pass != nil && pass.TypesInfo != nil {
		targetObj = pass.TypesInfo.Uses[cfgIdent]
		if targetObj == nil {
			targetObj = pass.TypesInfo.Defs[cfgIdent]
		}
	}

	rootBlock := enclosingFunc.Body
	if pass == nil {
		rootBlock = findDominatingBlock(enclosingFunc.Body, cfgIdent.Pos(), cfgIdent.Name)
	}

	finalState, _ := evalBlockStmtFlow(pass, file, rootBlock, call, targetObj, cfgIdent.Name, zeroStatus)
	return finalState
}

func evalStmtFlow(pass *analysis.Pass, file *ast.File, stmt ast.Stmt, call *ast.CallExpr, targetObj types.Object, varName string, inState ConfigStatus) (ConfigStatus, bool) {
	if stmt == nil {
		return inState, false
	}
	if call.Pos() < stmt.Pos() {
		return inState, true
	}

	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if s.Pos() <= call.Pos() && call.Pos() <= s.End() {
			return inState, true
		}
		evalAssignStmt(pass, file, s, targetObj, varName, &inState)
		return inState, false

	case *ast.ExprStmt:
		if s.Pos() <= call.Pos() && call.Pos() <= s.End() {
			return inState, true
		}
		if callExpr, ok := s.X.(*ast.CallExpr); ok {
			checkHelperCall(callExpr, pass, targetObj, varName, &inState)
		}
		return inState, false

	case *ast.IfStmt:
		if s.Init != nil {
			var reached bool
			inState, reached = evalStmtFlow(pass, file, s.Init, call, targetObj, varName, inState)
			if reached {
				return inState, true
			}
		}
		if s.Body != nil && s.Body.Pos() <= call.Pos() && call.Pos() <= s.Body.End() {
			return evalBlockStmtFlow(pass, file, s.Body, call, targetObj, varName, inState)
		}
		if s.Else != nil && s.Else.Pos() <= call.Pos() && call.Pos() <= s.Else.End() {
			return evalStmtFlow(pass, file, s.Else, call, targetObj, varName, inState)
		}

		thenState, _ := evalBlockStmtFlow(pass, file, s.Body, call, targetObj, varName, inState)
		thenTerm := isTerminating(s.Body)

		if s.Else != nil {
			elseState, _ := evalStmtFlow(pass, file, s.Else, call, targetObj, varName, inState)
			elseTerm := isTerminating(s.Else)

			if thenTerm && !elseTerm {
				return elseState, false
			}
			if elseTerm && !thenTerm {
				return thenState, false
			}
			return meetStatus(thenState, elseState), false
		}

		if thenTerm {
			return inState, false
		}
		return meetStatus(inState, thenState), false

	case *ast.BlockStmt:
		return evalBlockStmtFlow(pass, file, s, call, targetObj, varName, inState)

	case *ast.DeclStmt:
		if gen, ok := s.Decl.(*ast.GenDecl); ok {
			for _, spec := range gen.Specs {
				if valSpec, ok := spec.(*ast.ValueSpec); ok {
					for i, name := range valSpec.Names {
						if isSameTarget(pass, name, targetObj, varName) && i < len(valSpec.Values) {
							inState = evalConfigRHS(valSpec.Values[i])
						}
					}
				}
			}
		}
		return inState, false
	}

	return inState, false
}

func evalBlockStmtFlow(pass *analysis.Pass, file *ast.File, block *ast.BlockStmt, call *ast.CallExpr, targetObj types.Object, varName string, inState ConfigStatus) (ConfigStatus, bool) {
	if block == nil {
		return inState, false
	}
	currState := inState
	for _, stmt := range block.List {
		var reached bool
		currState, reached = evalStmtFlow(pass, file, stmt, call, targetObj, varName, currState)
		if reached {
			return currState, true
		}
	}
	return currState, false
}

func evalAssignStmt(pass *analysis.Pass, file *ast.File, assign *ast.AssignStmt, targetObj types.Object, varName string, status *ConfigStatus) {
	for i, lhs := range assign.Lhs {
		if !isSameTarget(pass, lhs, targetObj, varName) {
			continue
		}
		var rhsExpr ast.Expr
		if i < len(assign.Rhs) {
			rhsExpr = assign.Rhs[i]
		}

		if _, isIdent := lhs.(*ast.Ident); isIdent {
			if rhsExpr != nil {
				*status = evalConfigRHS(rhsExpr)
			}
			continue
		}

		lhsStr := exprToString(lhs)
		if strings.HasSuffix(lhsStr, ".MaxConnIdleTime") {
			status.HasMaxConnIdleTime = true
		}
		if strings.HasSuffix(lhsStr, ".MaxConnLifetime") {
			status.HasMaxConnLifetime = true
		}

		rhsVal := extractLitString(rhsExpr)

		if idxExpr, ok := lhs.(*ast.IndexExpr); ok {
			paramKey := extractLitString(idxExpr.Index)
			applyRuntimeParamMutation(paramKey, rhsVal, status)
		} else if strings.Contains(lhsStr, "statement_timeout") {
			applyRuntimeParamMutation("statement_timeout", rhsVal, status)
		} else if strings.Contains(lhsStr, "lock_timeout") {
			applyRuntimeParamMutation("lock_timeout", rhsVal, status)
		} else if strings.Contains(lhsStr, "idle_in_transaction") {
			applyRuntimeParamMutation("idle_in_transaction_session_timeout", rhsVal, status)
		}

		if rhsExpr != nil {
			evalRuntimeParamsExpr(rhsExpr, status)
		}
	}

	for _, rhsExpr := range assign.Rhs {
		if call, ok := rhsExpr.(*ast.CallExpr); ok {
			checkHelperCall(call, pass, targetObj, varName, status)
		}
	}
}

func evalConfigRHS(expr ast.Expr) ConfigStatus {
	var status ConfigStatus
	switch e := expr.(type) {
	case *ast.CompositeLit:
		return EvalCompositeLit(e)
	case *ast.UnaryExpr:
		if lit, ok := e.X.(*ast.CompositeLit); ok {
			return EvalCompositeLit(lit)
		}
	case *ast.CallExpr:
		return evalConfigCallExpr(e)
	case *ast.ParenExpr:
		return evalConfigRHS(e.X)
	}
	return status
}

func checkHelperCall(call *ast.CallExpr, pass *analysis.Pass, targetObj types.Object, varName string, status *ConfigStatus) {
	for _, arg := range call.Args {
		if isSameTarget(pass, arg, targetObj, varName) {
			fnName := exprToString(call.Fun)
			if strings.Contains(fnName, "configurePostgresPool") || strings.Contains(fnName, "Configure") {
				status.HasStatementTimeout = true
				status.HasLockTimeout = true
				status.HasIdleInTransaction = true
				status.HasMaxConnIdleTime = true
				status.HasMaxConnLifetime = true
			}
			if strings.Contains(fnName, "setRuntimeParam") && len(call.Args) >= 2 {
				paramName := extractLitString(call.Args[1])
				applyRuntimeParamMutation(paramName, "1000", status)
			}
		}
	}
}
