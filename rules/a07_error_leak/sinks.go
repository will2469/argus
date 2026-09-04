// Package a07_error_leak identifies HTTP/API response sink expressions where data is transmitted to external clients.
package a07_error_leak

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// ResponseSinkInfo captures a recognized response sink and the arguments being emitted.
type ResponseSinkInfo struct {
	IsSink bool
	Args   []ast.Expr
}

// InspectResponseSink checks whether an AST call expression represents an HTTP or API response sink.
func InspectResponseSink(pass *analysis.Pass, call *ast.CallExpr, fn *ast.FuncDecl) ResponseSinkInfo {
	if call == nil {
		return ResponseSinkInfo{}
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ResponseSinkInfo{}
	}

	// 1. http.Error(w, errText, statusCode)
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "http" && sel.Sel.Name == "Error" {
		if len(call.Args) >= 2 && IsResponseWriter(pass, call.Args[0], fn) {
			return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{call.Args[1]}}
		}
	}

	// 2. response.ErrorJSON(w, statusCode, errText) or similar helpers
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "response" && (sel.Sel.Name == "ErrorJSON" || sel.Sel.Name == "Error") {
		if len(call.Args) >= 3 && IsResponseWriter(pass, call.Args[0], fn) {
			return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{call.Args[2]}}
		} else if len(call.Args) >= 2 && IsResponseWriter(pass, call.Args[0], fn) {
			return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{call.Args[1]}}
		}
	}

	// 3. w.Write([]byte(...))
	if sel.Sel.Name == "Write" && len(call.Args) >= 1 {
		if IsResponseWriter(pass, sel.X, fn) {
			if byteConv, ok := call.Args[0].(*ast.CallExpr); ok && len(byteConv.Args) >= 1 {
				return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{byteConv.Args[0]}}
			}
			return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{call.Args[0]}}
		}
	}

	// 4. json.NewEncoder(w).Encode(data)
	if sel.Sel.Name == "Encode" && len(call.Args) >= 1 {
		if innerCall, ok := sel.X.(*ast.CallExpr); ok {
			if innerSel, ok := innerCall.Fun.(*ast.SelectorExpr); ok && innerSel.Sel.Name == "NewEncoder" && len(innerCall.Args) >= 1 {
				if IsResponseWriter(pass, innerCall.Args[0], fn) {
					return ResponseSinkInfo{IsSink: true, Args: []ast.Expr{call.Args[0]}}
				}
			}
		}
	}

	// 5. fmt.Fprintf(w, "%s", errText) or fmt.Fprint(w, errText)
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == "fmt" && len(call.Args) >= 1 {
		if IsResponseWriter(pass, call.Args[0], fn) {
			if sel.Sel.Name == "Fprintf" && len(call.Args) >= 3 {
				return ResponseSinkInfo{IsSink: true, Args: call.Args[2:]}
			} else if (sel.Sel.Name == "Fprint" || sel.Sel.Name == "Fprintln") && len(call.Args) >= 2 {
				return ResponseSinkInfo{IsSink: true, Args: call.Args[1:]}
			}
		}
	}

	return ResponseSinkInfo{}
}

// IsResponseWriter determines whether an expression is a valid HTTP/API response writer,
// strictly distinguishing it from arbitrary io.Writer implementations such as bytes.Buffer or os.File.
func IsResponseWriter(pass *analysis.Pass, expr ast.Expr, fn *ast.FuncDecl) bool {
	if expr == nil {
		return false
	}

	// 1. Explicitly reject address-of expressions (e.g. &buf, &buffer, &builder)
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		return false
	}

	// 2. Reject known buffer/file identifiers immediately
	if id, ok := expr.(*ast.Ident); ok {
		lower := strings.ToLower(id.Name)
		switch lower {
		case "buf", "buffer", "sb", "builder", "file", "f", "out", "pipe", "h", "hasher", "hash", "discard", "b", "stdout", "stderr":
			return false
		}
	}

	// 3. Semantic type resolution via go/types
	if pass != nil && pass.TypesInfo != nil {
		if t := pass.TypesInfo.TypeOf(expr); t != nil {
			tStr := t.String()
			if strings.Contains(tStr, "bytes.Buffer") ||
				strings.Contains(tStr, "os.File") ||
				strings.Contains(tStr, "strings.Builder") ||
				strings.Contains(tStr, "hash.Hash") {
				return false
			}
			if strings.Contains(tStr, "net/http.ResponseWriter") || strings.Contains(tStr, "ResponseWriter") {
				return true
			}
			if hasWriteHeaderMethod(t) {
				return true
			}
		}
	}

	// 4. AST fallback: verify enclosing function parameter types
	if fn != nil && fn.Type != nil && fn.Type.Params != nil {
		if id, ok := expr.(*ast.Ident); ok {
			for _, field := range fn.Type.Params.List {
				for _, name := range field.Names {
					if name.Name == id.Name {
						typeStr := types.ExprString(field.Type)
						if strings.Contains(typeStr, "ResponseWriter") {
							return true
						}
					}
				}
			}
		}
	}

	// 5. AST fallback: conventional HTTP response writer naming in handlers
	if id, ok := expr.(*ast.Ident); ok {
		switch id.Name {
		case "w", "rw", "res", "resp", "writer":
			return true
		}
	}

	// 6. Selector wrapper (e.g. struct fields like a.w or r.w or c.Writer)
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		name := sel.Sel.Name
		if name == "w" || name == "rw" || name == "Writer" || strings.Contains(name, "ResponseWriter") {
			return true
		}
	}

	return false
}

func hasWriteHeaderMethod(t types.Type) bool {
	if t == nil {
		return false
	}
	mset := types.NewMethodSet(t)
	for i := 0; i < mset.Len(); i++ {
		if mset.At(i).Obj().Name() == "WriteHeader" {
			return true
		}
	}
	if _, ok := t.(*types.Pointer); !ok {
		ptrMset := types.NewMethodSet(types.NewPointer(t))
		for i := 0; i < ptrMset.Len(); i++ {
			if ptrMset.At(i).Obj().Name() == "WriteHeader" {
				return true
			}
		}
	}
	return false
}
