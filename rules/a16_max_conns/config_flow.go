// Package a16_max_conns tracks sequential statement-level CFG flow and lattice joins for pgxpool.Config.
package a16_max_conns

import (
	"go/ast"
	"go/token"
	"strings"
)

// ConfigEvaluationResult holds the evaluated MaxConns status of a pgxpool.Config.
type ConfigEvaluationResult struct {
	HasMaxConns bool
	Valid       bool
	Message     string
	ReportNode  ast.Node
}

// ConfigFlowState holds intermediate lattice state during statement-level CFG tracking.
type ConfigFlowState struct {
	Configured bool
	Valid      bool
	Message    string
	ReportNode ast.Node
}

// TrackConfig evaluates the MaxConns configuration for a pgxpool.Config expression at call site.
func TrackConfig(cfgExpr ast.Expr, call *ast.CallExpr, file *ast.File, maxSafe int32) ConfigEvaluationResult {
	if composite, ok := unwrapCompositeLit(cfgExpr); ok {
		return evaluateCompositeLit(file, composite, maxSafe)
	}

	varName := ""
	if ident, ok := cfgExpr.(*ast.Ident); ok {
		varName = ident.Name
	}

	if varName == "" || file == nil || call == nil {
		return ConfigEvaluationResult{
			HasMaxConns: false,
			Valid:       false,
			Message:     "pgxpool.Config missing explicit MaxConns; relying on default (4 * NumCPU) risks connection starvation",
			ReportNode:  cfgExpr,
		}
	}

	fn := findEnclosingFunc(file, call.Pos())
	if fn == nil || fn.Body == nil {
		return ConfigEvaluationResult{
			HasMaxConns: false,
			Valid:       false,
			Message:     "pgxpool.Config missing explicit MaxConns; relying on default (4 * NumCPU) risks connection starvation",
			ReportNode:  cfgExpr,
		}
	}

	if fn.Name != nil && (strings.HasPrefix(fn.Name.Name, "ParseConfig") || strings.HasPrefix(fn.Name.Name, "DefaultConfig")) {
		return ConfigEvaluationResult{HasMaxConns: true, Valid: true}
	}

	finalState, _ := EvalBlockFlow(file, fn.Body, call.Pos(), varName, maxSafe, ConfigFlowState{})
	if !finalState.Configured {
		msg := finalState.Message
		if msg == "" {
			msg = "pgxpool.Config missing explicit MaxConns; set explicit value (e.g. 10-25 per pod) to prevent connection exhaustion"
		}
		return ConfigEvaluationResult{
			HasMaxConns: false,
			Valid:       false,
			Message:     msg,
			ReportNode:  cfgExpr,
		}
	}

	if !finalState.Valid {
		return ConfigEvaluationResult{
			HasMaxConns: true,
			Valid:       false,
			Message:     finalState.Message,
			ReportNode:  finalState.ReportNode,
		}
	}

	return ConfigEvaluationResult{HasMaxConns: true, Valid: true}
}

// EvalBlockFlow walks statements in block sequentially up to targetPos (or entire block if targetPos is NoPos).
func EvalBlockFlow(file *ast.File, block *ast.BlockStmt, targetPos token.Pos, varName string, maxSafe int32, in ConfigFlowState) (ConfigFlowState, bool) {
	if block == nil {
		return in, false
	}
	curr := in
	for _, stmt := range block.List {
		var reached bool
		curr, reached = evalStmtFlow(file, stmt, targetPos, varName, maxSafe, curr)
		if reached {
			return curr, true
		}
	}
	return curr, false
}

func evalStmtFlow(file *ast.File, stmt ast.Stmt, targetPos token.Pos, varName string, maxSafe int32, in ConfigFlowState) (ConfigFlowState, bool) {
	if stmt == nil {
		return in, false
	}
	if targetPos != token.NoPos && targetPos < stmt.Pos() {
		return in, true
	}

	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if targetPos != token.NoPos && s.Pos() <= targetPos && targetPos <= s.End() {
			return in, true
		}
		return evalAssignStmt(file, s, varName, maxSafe, in), false

	case *ast.ExprStmt:
		if targetPos != token.NoPos && s.Pos() <= targetPos && targetPos <= s.End() {
			return in, true
		}
		if callExpr, ok := s.X.(*ast.CallExpr); ok {
			return evalHelperCall(file, callExpr, varName, maxSafe, in), false
		}

	case *ast.IfStmt:
		if s.Init != nil {
			var reached bool
			in, reached = evalStmtFlow(file, s.Init, targetPos, varName, maxSafe, in)
			if reached {
				return in, true
			}
		}
		if targetPos != token.NoPos && s.Body != nil && s.Body.Pos() <= targetPos && targetPos <= s.Body.End() {
			return EvalBlockFlow(file, s.Body, targetPos, varName, maxSafe, in)
		}
		if targetPos != token.NoPos && s.Else != nil && s.Else.Pos() <= targetPos && targetPos <= s.Else.End() {
			return evalStmtFlow(file, s.Else, targetPos, varName, maxSafe, in)
		}

		thenState, _ := EvalBlockFlow(file, s.Body, targetPos, varName, maxSafe, in)
		thenTerm := isTerminating(s.Body)

		var elseState ConfigFlowState
		elseTerm := false
		if s.Else != nil {
			elseState, _ = evalStmtFlow(file, s.Else, targetPos, varName, maxSafe, in)
			elseTerm = isTerminating(s.Else)
		} else {
			elseState = in
		}

		if thenTerm && !elseTerm {
			return elseState, false
		}
		if elseTerm && !thenTerm {
			return thenState, false
		}

		// Universal Path Completeness: meet operator (all paths must configure MaxConns safely)
		joined := ConfigFlowState{
			Configured: thenState.Configured && elseState.Configured,
			Valid:      thenState.Valid && elseState.Valid,
		}
		if !joined.Configured {
			joined.Message = "pgxpool.Config missing explicit MaxConns on all execution paths; set explicit value to prevent connection exhaustion"
		} else if !joined.Valid {
			if !thenState.Valid {
				joined.Message = thenState.Message
				joined.ReportNode = thenState.ReportNode
			} else {
				joined.Message = elseState.Message
				joined.ReportNode = elseState.ReportNode
			}
		}
		return joined, false
	}

	return in, false
}

func evalAssignStmt(file *ast.File, assign *ast.AssignStmt, varName string, maxSafe int32, in ConfigFlowState) ConfigFlowState {
	curr := in
	for i, lhs := range assign.Lhs {
		sel, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Name != varName || sel.Sel.Name != "MaxConns" {
			continue
		}
		if i < len(assign.Rhs) {
			eval := EvaluateExprWithFile(file, assign.Rhs[i], maxSafe)
			curr.Configured = true
			curr.Valid = eval.Valid
			curr.Message = eval.Message
			curr.ReportNode = assign
		}
	}

	for _, rhs := range assign.Rhs {
		if call, ok := rhs.(*ast.CallExpr); ok {
			curr = evalHelperCall(file, call, varName, maxSafe, curr)
		}
	}
	return curr
}

func evalHelperCall(file *ast.File, call *ast.CallExpr, varName string, maxSafe int32, in ConfigFlowState) ConfigFlowState {
	for idx, arg := range call.Args {
		if id, ok := arg.(*ast.Ident); ok && id.Name == varName {
			if mutated, valid := inspectConfigMutatorHelper(file, call, idx, maxSafe); mutated {
				in.Configured, in.Valid, in.ReportNode = true, valid, call
				if !valid {
					in.Message = "pgxpool.Config MaxConns assigned invalid bounds inside helper function"
				}
			}
		}
	}
	return in
}

func isTerminating(stmt ast.Stmt) bool {
	if stmt == nil {
		return false
	}
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ExprStmt:
		if call, ok := s.X.(*ast.CallExpr); ok {
			name := ""
			switch f := call.Fun.(type) {
			case *ast.Ident:
				name = f.Name
			case *ast.SelectorExpr:
				name = f.Sel.Name
			}
			return name == "panic" || name == "Exit" || name == "Fatal" || name == "Fatalf"
		}
	case *ast.BlockStmt:
		return len(s.List) > 0 && isTerminating(s.List[len(s.List)-1])
	}
	return false
}
