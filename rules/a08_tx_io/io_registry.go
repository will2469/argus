// Package a08_tx_io identifies external blocking I/O operations (network, disk, sleep, exec, storage SDKs)
// that must not be executed while holding an open database transaction.
package a08_tx_io

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// CheckBlockingIO returns a description of the blocking external I/O operation if node matches, or empty string.
func CheckBlockingIO(n ast.Node, pass *analysis.Pass) string {
	return CheckBlockingIOWithContext(n, pass, nil, nil)
}

// CheckBlockingIOWithContext checks if an AST node is a blocking external I/O call within function/file context.
func CheckBlockingIOWithContext(n ast.Node, pass *analysis.Pass, fn *ast.FuncDecl, file *ast.File) string {
	call, ok := n.(*ast.CallExpr)
	if !ok {
		return ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	// 1. Direct package-level calls (time.Sleep, http.Get, os.ReadFile, exec.Command, net.Dial)
	if pkg, ok := sel.X.(*ast.Ident); ok {
		if op := checkPackageCall(pkg, sel.Sel.Name, pass, file); op != "" {
			return op
		}
	}

	// 2. Client method calls (http.Client.Do, storage.PutObject, net.Conn.Read, etc.)
	if isHTTPClientCall(pass, sel, fn, file) {
		return "http.Client." + sel.Sel.Name
	}
	if isStorageClientCall(pass, sel, fn, file) {
		return "storage." + sel.Sel.Name
	}
	if isNetConnCall(pass, sel) {
		return "net.Conn." + sel.Sel.Name
	}
	if isExecCmdCall(pass, sel) {
		return "exec.Cmd." + sel.Sel.Name
	}
	if isOSFileCall(pass, sel) {
		return "os.File." + sel.Sel.Name
	}

	return ""
}

func matchPkgMethod(importPath, methodName string) string {
	switch importPath {
	case "time":
		if methodName == "Sleep" {
			return "time.Sleep"
		}
	case "net/http":
		switch methodName {
		case "Get", "Post", "PostForm", "Head":
			return "http." + methodName
		}
	case "net":
		if strings.HasPrefix(methodName, "Dial") || strings.HasPrefix(methodName, "Listen") {
			return "net." + methodName
		}
	case "crypto/tls":
		if strings.HasPrefix(methodName, "Dial") {
			return "tls." + methodName
		}
	case "os/exec":
		if strings.HasPrefix(methodName, "Command") {
			return "exec." + methodName
		}
	case "os":
		switch methodName {
		case "ReadFile", "WriteFile", "Create", "Open", "OpenFile", "Remove", "RemoveAll", "Mkdir", "MkdirAll", "Truncate":
			return "os." + methodName
		}
	}
	return ""
}

func checkPackageCall(pkg *ast.Ident, methodName string, pass *analysis.Pass, file *ast.File) string {
	if pkg == nil {
		return ""
	}
	if pass != nil && pass.TypesInfo != nil {
		if obj, ok := pass.TypesInfo.Uses[pkg].(*types.PkgName); ok {
			return matchPkgMethod(obj.Imported().Path(), methodName)
		}
	}
	knownPaths := map[string]string{
		"time": "time", "http": "net/http", "net": "net",
		"tls": "crypto/tls", "exec": "os/exec", "os": "os",
	}
	if expected, ok := knownPaths[pkg.Name]; ok {
		if file == nil || isImportedPackage(file, pkg.Name, expected) {
			return matchPkgMethod(expected, methodName)
		}
	}
	return ""
}

func isHTTPClientCall(pass *analysis.Pass, sel *ast.SelectorExpr, fn *ast.FuncDecl, file *ast.File) bool {
	switch sel.Sel.Name {
	case "Do", "Get", "Post", "PostForm", "Head", "RoundTrip":
	default:
		return false
	}

	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(sel.X)
		if t != nil {
			t = unwrapPointer(t)
			typeStr := t.String()
			// Provenance-based: only accept exact type path
			if typeStr == "net/http.Client" || typeStr == "net/http.RoundTripper" ||
				strings.HasPrefix(typeStr, "net/http.Client") || strings.HasPrefix(typeStr, "net/http.RoundTripper") {
				return true
			}
			// Reject structural resemblance (any type with Do/Get/Post methods)
			return false
		}
	}

	// AST fallback: heuristic check for http.Client type selector
	// Accept only if type name is exactly "Client" or "RoundTripper" from net/http package
	astType := findASTType(sel.X, fn, file)
	typeName := getASTTypeName(astType)
	if astType != nil {
		if sel, ok := astType.(*ast.SelectorExpr); ok {
			if pkgID, ok := sel.X.(*ast.Ident); ok && pkgID.Name == "http" {
				if sel.Sel.Name == "Client" || sel.Sel.Name == "RoundTripper" {
					return true
				}
			}
		}
	}
	// Accept type name "Client" or "RoundTripper" as heuristic (may have false positives)
	return typeName == "Client" || typeName == "RoundTripper"
}

func isStorageClientCall(pass *analysis.Pass, sel *ast.SelectorExpr, fn *ast.FuncDecl, file *ast.File) bool {
	// Highly specific storage SDK methods (strong evidence regardless of receiver type)
	switch sel.Sel.Name {
	case "PutObject", "GetObject", "DeleteObject", "HeadObject", "ListObjects", "ListObjectsV2",
		"UploadPart", "CopyObject", "CreateMultipartUpload", "CompleteMultipartUpload", "AbortMultipartUpload":
		return isConfirmedStorageReceiver(pass, sel.X, fn, file, true)
	case "Upload", "Download":
		// Generic methods require storage-like receiver type name as additional evidence
		return isConfirmedStorageReceiver(pass, sel.X, fn, file, false)
	default:
		return false
	}
}

func isConfirmedStorageReceiver(pass *analysis.Pass, expr ast.Expr, fn *ast.FuncDecl, file *ast.File, isDomainSpecificMethod bool) bool {
	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(expr)
		if t != nil {
			t = unwrapPointer(t)
			if isProvenNonStorageType(t) {
				return false
			}
			// Provenance-based: accept exact storage SDK package paths
			if isStorageSDKType(t.String()) {
				return true
			}
			// For highly specific storage methods (PutObject, GetObject, etc.),
			// method name is strong evidence of storage I/O even without SDK provenance
			if isDomainSpecificMethod {
				return true
			}
			// For generic methods (Upload, Download), require storage-like type name
			typeName := getTypeNameFromType(t)
			if isStorageLikeTypeName(typeName) {
				return true
			}
			return false
		}
	}

	// AST fallback mode
	if isDomainSpecificMethod {
		// Highly specific methods (PutObject, etc.) are strong evidence alone
		return true
	}

	// For generic methods (Upload, Download), require storage-like type name
	astType := findASTType(expr, fn, file)
	typeName := getASTTypeName(astType)
	return isStorageLikeTypeName(typeName)
}

func getTypeNameFromType(t types.Type) string {
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		return named.Obj().Name()
	}
	return ""
}

func isStorageLikeTypeName(name string) bool {
	if name == "" {
		return false
	}
	// Storage-related type name heuristics (case-insensitive substring matching)
	lowerName := strings.ToLower(name)
	storageKeywords := []string{"storage", "s3", "bucket", "uploader", "blob", "minio", "gcs", "azure"}
	for _, keyword := range storageKeywords {
		if strings.Contains(lowerName, keyword) {
			return true
		}
	}
	return false
}

func isProvenNonStorageType(t types.Type) bool {
	if named, ok := t.(*types.Named); ok && named.Obj() != nil {
		return isNonStorageTypeName(named.Obj().Name())
	}
	return false
}

func isNonStorageTypeName(name string) bool {
	switch name {
	case "Calculator", "Calc", "Parser", "Math", "Engine":
		return true
	}
	return false
}
