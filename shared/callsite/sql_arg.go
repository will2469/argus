// Package callsite provides AST recognition utilities for database operations.
package callsite

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// ExtractSQLArg extracts the exact AST expression corresponding to the SQL query
// from arguments of a database operation, ignoring context parameters and bound arguments.
func ExtractSQLArg(call *ast.CallExpr, pass *analysis.Pass) ast.Expr {
	if call == nil || len(call.Args) == 0 {
		return nil
	}

	methodName := GetCallMethodName(call.Fun)

	// 1. Methods that do NOT take a SQL query string argument
	switch methodName {
	case "SendBatch", "CopyFrom", "Begin", "BeginTx", "BeginFunc", "Commit", "Rollback":
		return nil
	case "Queue":
		// pgx.Batch.Queue(query string, args ...any)
		// Argument 0 is the SQL query expression
		return call.Args[0]
	}

	// 2. In database/sql and standard drivers, *Context methods always take context as arg 0.
	if strings.HasSuffix(methodName, "Context") {
		if len(call.Args) >= 2 {
			return call.Args[1]
		}
		return nil
	}

	if IsContextArg(call.Args[0], pass) {
		if len(call.Args) >= 2 {
			return call.Args[1]
		}
		return nil
	}

	return call.Args[0]
}

// IsContextArg checks whether an AST expression represents a context.Context argument.
func IsContextArg(arg ast.Expr, pass *analysis.Pass) bool {
	if arg == nil {
		return false
	}

	// 1. Literal strings or string concatenations are never context
	switch e := arg.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return false
		}
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return false
		}
	}

	// 2. Type-aware check via go/types when pass is available
	if pass != nil && pass.TypesInfo != nil {
		if tv, ok := pass.TypesInfo.Types[arg]; ok && tv.Type != nil {
			if isContextType(tv.Type) {
				return true
			}
		}
		if id, ok := arg.(*ast.Ident); ok {
			if obj := pass.TypesInfo.Uses[id]; obj != nil && isContextType(obj.Type()) {
				return true
			}
		}
	}

	// 3. Syntactic AST heuristics (standalone runner / untyped AST mode)
	if id, ok := arg.(*ast.Ident); ok {
		lower := strings.ToLower(id.Name)
		return lower == "ctx" || lower == "context" ||
			strings.HasPrefix(lower, "ctx") ||
			strings.HasSuffix(lower, "ctx") ||
			strings.HasSuffix(lower, "context")
	}

	if call, ok := arg.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "context" {
				return true
			}
			lower := strings.ToLower(sel.Sel.Name)
			if lower == "context" || lower == "background" || lower == "todo" {
				return true
			}
		}
	}

	return false
}

func isContextType(t types.Type) bool {
	if t == nil {
		return false
	}
	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context" {
			return true
		}
	}
	if iface, ok := t.Underlying().(*types.Interface); ok {
		hasDeadline, hasDone, hasErr, hasValue := false, false, false, false
		for i := 0; i < iface.NumMethods(); i++ {
			switch iface.Method(i).Name() {
			case "Deadline":
				hasDeadline = true
			case "Done":
				hasDone = true
			case "Err":
				hasErr = true
			case "Value":
				hasValue = true
			}
		}
		if hasDeadline && hasDone && hasErr && hasValue {
			return true
		}
	}
	return false
}
