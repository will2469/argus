// Package a07_error_leak identifies HTTP/API response sink expressions where data is transmitted to external clients.
package a07_error_leak

import (
	"go/ast"
)

// ResponseSinkInfo captures a recognized response sink and the arguments being emitted.
type ResponseSinkInfo struct {
	IsSink bool
	Args   []ast.Expr
}

// InspectResponseSink checks whether an AST call expression represents an HTTP or API response sink.
func InspectResponseSink(call *ast.CallExpr) ResponseSinkInfo {
	if call == nil {
		return ResponseSinkInfo{}
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ResponseSinkInfo{}
	}

	// 1. http.Error(w, errText, statusCode)
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "http" && sel.Sel.Name == "Error" {
		if len(call.Args) >= 2 {
			return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{call.Args[1]}}
		}
	}

	// 2. response.ErrorJSON(w, statusCode, errText) or similar helpers
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "response" && (sel.Sel.Name == "ErrorJSON" || sel.Sel.Name == "Error") {
		if len(call.Args) >= 3 {
			return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{call.Args[2]}}
		} else if len(call.Args) >= 2 {
			return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{call.Args[1]}}
		}
	}

	// 3. w.Write([]byte(...))
	if sel.Sel.Name == "Write" && len(call.Args) >= 1 {
		if byteConv, ok := call.Args[0].(*ast.CallExpr); ok && len(byteConv.Args) >= 1 {
			return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{byteConv.Args[0]}}
		}
		return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{call.Args[0]}}
	}

	// 4. json.NewEncoder(w).Encode(data)
	if sel.Sel.Name == "Encode" && len(call.Args) >= 1 {
		if innerCall, ok := sel.X.(*ast.CallExpr); ok {
			if innerSel, ok := innerCall.Fun.(*ast.SelectorExpr); ok && innerSel.Sel.Name == "NewEncoder" {
				return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{call.Args[0]}}
			}
		}
	}

	// 5. fmt.Fprintf(w, "%s", errText) or fmt.Fprint(w, errText)
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "fmt" {
		if sel.Sel.Name == "Fprintf" && len(call.Args) >= 3 {
			return ResponseSinkInfo{IsSink: true, Args: call.Args[2:]}
		} else if (sel.Sel.Name == "Fprint" || sel.Sel.Name == "Fprintln") && len(call.Args) >= 2 {
			return ResponseSinkInfo{IsSink: true, Args: call.Args[1:]}
		}
	}

	return ResponseSinkInfo{}
}
