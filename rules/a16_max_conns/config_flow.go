// Package a16_max_conns tracks sequential statement-level CFG flow and lattice joins for pgxpool.Config.
package a16_max_conns

import (
	"go/ast"
	"strings"
)

// ConfigEvaluationResult holds the evaluated MaxConns status of a pgxpool.Config.
type ConfigEvaluationResult struct {
	HasMaxConns bool
	Valid       bool
	Message     string
	ReportNode  ast.Node
}

type configState struct {
	configured bool
	valid      bool
	message    string
	reportNode ast.Node
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

	finalState, _ := evalBlockStmtFlow(file, fn.Body, call, varName, maxSafe, configState{})
	if !finalState.configured {
		msg := finalState.message
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

	if !finalState.valid {
		return ConfigEvaluationResult{
			HasMaxConns: true,
			Valid:       false,
			Message:     finalState.message,
			ReportNode:  finalState.reportNode,
		}
	}

	return ConfigEvaluationResult{HasMaxConns: true, Valid: true}
}

func evalBlockStmtFlow(file *ast.File, block *ast.BlockStmt, call *ast.CallExpr, varName string, maxSafe int32, in configState) (configState, bool) {
	if block == nil {
		return in, false
	}
	curr := in
	for _, stmt := range block.List {
		var reached bool
		curr, reached = evalStmtFlow(file, stmt, call, varName, maxSafe, curr)
		if reached {
			return curr, true
		}
	}
	return curr, false
}

func evalStmtFlow(file *ast.File, stmt ast.Stmt, call *ast.CallExpr, varName string, maxSafe int32, in configState) (configState, bool) {
	if stmt == nil {
		return in, false
	}
	if call.Pos() < stmt.Pos() {
		return in, true
	}

	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if s.Pos() <= call.Pos() && call.Pos() <= s.End() {
			return in, true
		}
		return evalAssignStmt(file, s, varName, maxSafe, in), false

	case *ast.ExprStmt:
		if s.Pos() <= call.Pos() && call.Pos() <= s.End() {
			return in, true
		}
		if callExpr, ok := s.X.(*ast.CallExpr); ok {
			return evalHelperCall(file, callExpr, varName, maxSafe, in), false
		}

	case *ast.IfStmt:
		if s.Init != nil {
			var reached bool
			in, reached = evalStmtFlow(file, s.Init, call, varName, maxSafe, in)
			if reached {
				return in, true
			}
		}
		if s.Body != nil && s.Body.Pos() <= call.Pos() && call.Pos() <= s.Body.End() {
			return evalBlockStmtFlow(file, s.Body, call, varName, maxSafe, in)
		}
		if s.Else != nil && s.Else.Pos() <= call.Pos() && call.Pos() <= s.Else.End() {
			return evalStmtFlow(file, s.Else, call, varName, maxSafe, in)
		}

		thenState, _ := evalBlockStmtFlow(file, s.Body, call, varName, maxSafe, in)
		thenTerm := isTerminating(s.Body)

		var elseState configState
		elseTerm := false
		if s.Else != nil {
			elseState, _ = evalStmtFlow(file, s.Else, call, varName, maxSafe, in)
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
		joined := configState{
			configured: thenState.configured && elseState.configured,
			valid:      thenState.valid && elseState.valid,
		}
		if !joined.configured {
			joined.message = "pgxpool.Config missing explicit MaxConns on all execution paths; set explicit value to prevent connection exhaustion"
		} else if !joined.valid {
			if !thenState.valid {
				joined.message = thenState.message
				joined.reportNode = thenState.reportNode
			} else {
				joined.message = elseState.message
				joined.reportNode = elseState.reportNode
			}
		}
		return joined, false
	}

	return in, false
}

func evalAssignStmt(file *ast.File, assign *ast.AssignStmt, varName string, maxSafe int32, in configState) configState {
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
			curr.configured = true
			curr.valid = eval.Valid
			curr.message = eval.Message
			curr.reportNode = assign
		}
	}

	for _, rhs := range assign.Rhs {
		if call, ok := rhs.(*ast.CallExpr); ok {
			curr = evalHelperCall(file, call, varName, maxSafe, curr)
		}
	}
	return curr
}

func evalHelperCall(file *ast.File, call *ast.CallExpr, varName string, maxSafe int32, in configState) configState {
	for idx, arg := range call.Args {
		if id, ok := arg.(*ast.Ident); ok && id.Name == varName {
			if mutated, valid := inspectConfigMutatorHelper(file, call, idx, maxSafe); mutated {
				in.configured = true
				in.valid = valid
				in.reportNode = call
				if !valid {
					in.message = "pgxpool.Config MaxConns assigned invalid bounds inside helper function"
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
		if len(s.List) == 0 {
			return false
		}
		return isTerminating(s.List[len(s.List)-1])
	}
	return false
}
