// Package a08_tx_io identifies external blocking I/O operations (network, disk, sleep, exec, locks)
// that must not be executed while holding an open database transaction.
package a08_tx_io

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// CheckBlockingIO returns a description of the blocking I/O operation if node matches, or empty string.
func CheckBlockingIO(n ast.Node, pass *analysis.Pass) string {
	switch node := n.(type) {
	case *ast.CallExpr:
		return checkCallExprBlockingIO(node, pass)
	case *ast.SendStmt:
		return "channel send"
	case *ast.UnaryExpr:
		if node.Op.String() == "<-" {
			return "channel receive"
		}
	}
	return ""
}

func checkCallExprBlockingIO(call *ast.CallExpr, pass *analysis.Pass) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	// 1. Direct package-level calls (time.Sleep, http.Get, os.ReadFile, exec.Command)
	if pkg, ok := sel.X.(*ast.Ident); ok {
		switch pkg.Name {
		case "time":
			if sel.Sel.Name == "Sleep" {
				return "time.Sleep"
			}
		case "http":
			switch sel.Sel.Name {
			case "Get", "Post", "PostForm", "Head":
				return "http." + sel.Sel.Name
			}
		case "net":
			if strings.HasPrefix(sel.Sel.Name, "Dial") || strings.HasPrefix(sel.Sel.Name, "Listen") {
				return "net." + sel.Sel.Name
			}
		case "tls":
			if strings.HasPrefix(sel.Sel.Name, "Dial") {
				return "tls." + sel.Sel.Name
			}
		case "exec":
			if strings.HasPrefix(sel.Sel.Name, "Command") {
				return "exec." + sel.Sel.Name
			}
		case "os":
			switch sel.Sel.Name {
			case "ReadFile", "WriteFile", "Create", "Open", "OpenFile", "Remove", "RemoveAll":
				return "os." + sel.Sel.Name
			}
		}
	}

	// 2. Client method calls (client.Do, s3.PutObject, mutex.Lock)
	methodName := sel.Sel.Name
	switch methodName {
	case "Do":
		if pass != nil && pass.TypesInfo != nil {
			typ := pass.TypesInfo.TypeOf(sel.X)
			if typ != nil && strings.Contains(typ.String(), "net/http.Client") {
				return "http.Client.Do"
			}
		}
		// AST heuristic fallback
		if id, ok := sel.X.(*ast.Ident); ok && (strings.Contains(id.Name, "http") || strings.Contains(id.Name, "client")) {
			return "http.Client.Do"
		}

	case "PutObject", "GetObject", "Upload":
		return "storage." + methodName

	case "Lock", "RLock":
		if id, ok := sel.X.(*ast.Ident); ok && (strings.Contains(id.Name, "mu") || strings.Contains(id.Name, "mutex") || strings.Contains(id.Name, "lock")) {
			return "sync.Mutex." + methodName
		}
	}

	return ""
}
