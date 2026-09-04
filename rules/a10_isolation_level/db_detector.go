// Package a10_isolation_level provides semantic receiver verification for database pools and transactions.
package a10_isolation_level

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/callsite"
)

// isDBReceiver verifies whether a call expression is invoked on a proven database connection or pool.
func isDBReceiver(pass *analysis.Pass, fun ast.Expr, fn *ast.FuncDecl, file *ast.File) bool {
	sel := callsite.GetCallSelector(fun)
	if sel == nil {
		return false
	}
	return isDBExpr(pass, sel.X, fn, file)
}

// isDBExpr checks whether an arbitrary AST expression represents a proven database pool or connection.
func isDBExpr(pass *analysis.Pass, expr ast.Expr, fn *ast.FuncDecl, file *ast.File) bool {
	if expr == nil {
		return false
	}

	// 1. Semantic Type Checking via pass.TypesInfo
	if pass != nil && pass.TypesInfo != nil {
		var recvType types.Type
		if tv, ok := pass.TypesInfo.Types[expr]; ok && tv.Type != nil {
			recvType = tv.Type
		} else if id, ok := expr.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil {
				recvType = obj.Type()
			} else if obj := pass.TypesInfo.Defs[id]; obj != nil {
				recvType = obj.Type()
			} else if obj := pass.TypesInfo.ObjectOf(id); obj != nil {
				recvType = obj.Type()
			}
		}

		if recvType != nil && recvType != types.Typ[types.Invalid] {
			if isProvenDBPoolInterface(recvType) {
				return true
			}
			if !hasInvalidType(recvType) {
				return false
			}
		}
	}

	// 2. AST-level Symbol / Type Verification (pass == nil or unresolved)
	if id, ok := expr.(*ast.Ident); ok {
		if isKnownDBPackage(id.Name, file) && !isIdentShadowed(file, fn, id.Pos(), id.Name) {
			return true
		}
	}

	// Resolve receiver declared type in AST
	astType := findASTType(expr, fn, file)
	if astType != nil && isProvenDBPoolASTType(astType, file) {
		return true
	}

	// Check if assigned from a DB constructor
	if id, ok := expr.(*ast.Ident); ok && id.Obj != nil {
		if as, ok := id.Obj.Decl.(*ast.AssignStmt); ok {
			for i, lhs := range as.Lhs {
				if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name {
					var rhs ast.Expr
					if i < len(as.Rhs) {
						rhs = as.Rhs[i]
					} else if len(as.Rhs) == 1 {
						rhs = as.Rhs[0]
					}
					if rhsCall, ok := rhs.(*ast.CallExpr); ok && isDBPoolConstructorCall(rhsCall, file) {
						return true
					}
				}
			}
		}
	}

	return false
}

// isProvenTxHelperCall verifies whether a call is a genuine database transaction helper invocation,
// rejecting non-DB methods that happen to be named WithTx, BeginFunc, or ExecuteTx.
func isProvenTxHelperCall(pass *analysis.Pass, call *ast.CallExpr, fn *ast.FuncDecl, file *ast.File) bool {
	if call == nil {
		return false
	}
	methodName := callsite.GetCallMethodName(call.Fun)
	if methodName != "WithTx" && methodName != "BeginFunc" && methodName != "ExecuteTx" {
		return false
	}

	// A. If receiver itself is a proven DB connection/pool (e.g. pool.BeginFunc)
	if isDBReceiver(pass, call.Fun, fn, file) {
		return extractClosureArg(call) != nil
	}

	// B. If a helper passes a proven DB connection/pool as an argument (e.g. argus.WithTx(ctx, pool, fn))
	var hasDBArg bool
	for _, arg := range call.Args {
		if isDBExpr(pass, arg, fn, file) {
			hasDBArg = true
			break
		}
	}
	if !hasDBArg {
		return false
	}

	closure := extractClosureArg(call)
	if closure == nil || closure.Type == nil || closure.Type.Params == nil || len(closure.Type.Params.List) == 0 {
		return false
	}

	if pass != nil && pass.TypesInfo != nil {
		firstParam := closure.Type.Params.List[0]
		if len(firstParam.Names) > 0 {
			if obj := pass.TypesInfo.Defs[firstParam.Names[0]]; obj != nil {
				if isProvenClosureTxType(obj.Type()) {
					return true
				}
				if !hasInvalidType(obj.Type()) {
					return false
				}
			}
		}
	}

	firstParam := closure.Type.Params.List[0]
	return isProvenClosureTxASTType(firstParam.Type, file)
}

func extractClosureArg(call *ast.CallExpr) *ast.FuncLit {
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.FuncLit); ok {
			return lit
		}
	}
	return nil
}

// isProvenTxReceiver verifies whether an expression is the expected transaction identifier
// or an object with proven database transaction capabilities.
func isProvenTxReceiver(pass *analysis.Pass, expr ast.Expr, expectedTxID *ast.Ident, body *ast.BlockStmt) bool {
	if expr == nil {
		return false
	}

	// 1. Direct match with expected transaction identifier via scope-hierarchy object identity
	if expectedTxID != nil {
		if id, ok := expr.(*ast.Ident); ok && isSameObject(pass, id, expectedTxID, body) {
			return true
		}
	}

	// 2. Semantic Type Checking via pass.TypesInfo
	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(expr)
		if t != nil && t != types.Typ[types.Invalid] {
			return isProvenDBTxType(t)
		}
	}

	return false
}
