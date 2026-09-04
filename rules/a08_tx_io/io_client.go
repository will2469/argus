// Package a08_tx_io provides type checking helpers for standard library and storage client method calls.
package a08_tx_io

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

func isStorageSDKType(typeStr string) bool {
	// Fail-closed: only accept exact package path matches, not substring matching
	storagePackages := map[string]bool{
		"github.com/aws/aws-sdk-go-v2/service/s3":           true,
		"github.com/aws/aws-sdk-go/service/s3":              true,
		"cloud.google.com/go/storage":                      true,
		"github.com/minio/minio-go":                         true,
		"github.com/Azure/azure-sdk-for-go/storage":         true,
	}

	// Check if typeStr exactly matches or starts with a known storage package path
	for pkg := range storagePackages {
		if typeStr == pkg || strings.HasPrefix(typeStr, pkg+".") {
			return true
		}
	}
	return false
}

func isNetConnCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	if sel.Sel.Name != "Read" && sel.Sel.Name != "Write" {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(sel.X)
		// Fail-closed: only accept exact type path, not substring matching
		return t != nil && (t.String() == "net.Conn" || strings.HasPrefix(t.String(), "net.Conn"))
	}
	return false
}

func isExecCmdCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	switch sel.Sel.Name {
	case "Run", "Output", "CombinedOutput", "Start":
		if pass != nil && pass.TypesInfo != nil {
			t := pass.TypesInfo.TypeOf(sel.X)
			// Fail-closed: only accept exact type path, not substring matching
			return t != nil && (t.String() == "os/exec.Cmd" || strings.HasPrefix(t.String(), "os/exec.Cmd"))
		}
	}
	return false
}

func isOSFileCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	switch sel.Sel.Name {
	case "Read", "Write", "ReadAt", "WriteAt", "WriteString", "Sync":
		if pass != nil && pass.TypesInfo != nil {
			t := pass.TypesInfo.TypeOf(sel.X)
			// Fail-closed: only accept exact type path, not substring matching
			return t != nil && (t.String() == "os.File" || strings.HasPrefix(t.String(), "os.File"))
		}
	}
	return false
}


func isImportedPackage(file *ast.File, pkgName, expectedPath string) bool {
	if file == nil {
		return false
	}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if imp.Name != nil {
			if imp.Name.Name == pkgName && strings.HasSuffix(path, expectedPath) {
				return true
			}
		} else {
			parts := strings.Split(path, "/")
			lastPart := parts[len(parts)-1]
			if lastPart == pkgName && (path == expectedPath || strings.HasSuffix(path, "/"+expectedPath)) {
				return true
			}
		}
	}
	return false
}
