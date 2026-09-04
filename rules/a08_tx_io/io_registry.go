// Package a08_tx_io identifies external blocking I/O operations (network, disk, sleep, exec, storage SDKs)
// that must not be executed while holding an open database transaction.
// Internal Go runtime synchronization (channels, mutexes) is deliberately excluded.
package a08_tx_io

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// CheckBlockingIO returns a description of the blocking external I/O operation if node matches, or empty string.
func CheckBlockingIO(n ast.Node, pass *analysis.Pass) string {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return ""
	}
	return checkCallExprBlockingIO(call, pass)
}

func checkCallExprBlockingIO(call *ast.CallExpr, pass *analysis.Pass) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	// 1. Direct package-level calls (time.Sleep, http.Get, os.ReadFile, exec.Command, net.Dial)
	if pkg, ok := sel.X.(*ast.Ident); ok {
		if op := checkPackageCall(pkg.Name, sel.Sel.Name); op != "" {
			return op
		}
	}

	// 2. Client method calls (http.Client.Do, storage.PutObject, etc.)
	if isHTTPClientCall(pass, sel) {
		return "http.Client." + sel.Sel.Name
	}

	if isStorageClientCall(pass, sel) {
		return "storage." + sel.Sel.Name
	}

	if isNetConnCall(pass, sel) {
		return "net.Conn." + sel.Sel.Name
	}

	return ""
}

func checkPackageCall(pkgName, methodName string) string {
	switch pkgName {
	case "time":
		if methodName == "Sleep" {
			return "time.Sleep"
		}
	case "http":
		switch methodName {
		case "Get", "Post", "PostForm", "Head":
			return "http." + methodName
		}
	case "net":
		if strings.HasPrefix(methodName, "Dial") || strings.HasPrefix(methodName, "Listen") {
			return "net." + methodName
		}
	case "tls":
		if strings.HasPrefix(methodName, "Dial") {
			return "tls." + methodName
		}
	case "exec":
		if strings.HasPrefix(methodName, "Command") {
			return "exec." + methodName
		}
	case "os":
		switch methodName {
		case "ReadFile", "WriteFile", "Create", "Open", "OpenFile", "Remove", "RemoveAll", "Mkdir", "MkdirAll":
			return "os." + methodName
		}
	}
	return ""
}

func isHTTPClientCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	methodName := sel.Sel.Name
	if methodName != "Do" && methodName != "Get" && methodName != "Post" && methodName != "Head" {
		return false
	}

	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(sel.X)
		if t != nil {
			typeStr := t.String()
			if strings.Contains(typeStr, "net/http.Client") || strings.Contains(typeStr, "net/http.RoundTripper") {
				return true
			}
			// Check interface method set for Do(*http.Request)
			if iface, ok := t.Underlying().(*types.Interface); ok {
				for i := 0; i < iface.NumMethods(); i++ {
					m := iface.Method(i)
					if m.Name() == "Do" {
						return true
					}
				}
			}
		}
	}

	// AST identifier heuristic fallback
	if id, ok := sel.X.(*ast.Ident); ok {
		lower := strings.ToLower(id.Name)
		return strings.Contains(lower, "http") || strings.Contains(lower, "client")
	}
	return false
}

func isStorageClientCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	methodName := sel.Sel.Name
	switch methodName {
	case "PutObject", "GetObject", "Upload", "Download", "DeleteObject":
	default:
		return false
	}

	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(sel.X)
		if t != nil {
			for {
				if ptr, ok := t.(*types.Pointer); ok {
					t = ptr.Elem()
				} else {
					break
				}
			}

			// Verify package path or type name belongs to genuine storage SDK
			typeStr := t.String()
			if isStorageSDKType(typeStr) {
				return true
			}

			if named, ok := t.(*types.Named); ok {
				if obj := named.Obj(); obj != nil {
					name := obj.Name()
					if strings.Contains(name, "Storage") || strings.Contains(name, "S3") ||
						strings.Contains(name, "Bucket") || strings.Contains(name, "Uploader") {
						return true
					}
				}
			}
			// If type is resolved and does NOT match storage SDK, it is NOT storage (e.g. calculator.Upload)
			return false
		}
	}

	// AST fallback: verify receiver name is storage-related, reject arbitrary words (calculator, calc, etc.)
	recvName := getReceiverName(sel.X)
	if recvName != "" {
		lower := strings.ToLower(recvName)
		if strings.Contains(lower, "calc") || strings.Contains(lower, "math") || strings.Contains(lower, "parser") {
			return false
		}
		return strings.Contains(lower, "storage") || strings.Contains(lower, "s3") ||
			strings.Contains(lower, "bucket") || strings.Contains(lower, "minio") ||
			strings.Contains(lower, "blob") || strings.Contains(lower, "upload")
	}

	return false
}

func isStorageSDKType(typeStr string) bool {
	storagePackages := []string{
		"github.com/aws/aws-sdk-go-v2/service/s3",
		"github.com/aws/aws-sdk-go/service/s3",
		"cloud.google.com/go/storage",
		"github.com/minio/minio-go",
		"github.com/Azure/azure-sdk-for-go",
	}
	for _, pkg := range storagePackages {
		if strings.Contains(typeStr, pkg) {
			return true
		}
	}
	return false
}

func isNetConnCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	methodName := sel.Sel.Name
	if methodName != "Read" && methodName != "Write" {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(sel.X)
		if t != nil && strings.Contains(t.String(), "net.Conn") {
			return true
		}
	}
	return false
}
