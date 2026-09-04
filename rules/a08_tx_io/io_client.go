// Package a08_tx_io provides type checking helpers for standard library and storage client method calls.
package a08_tx_io

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

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
	if sel.Sel.Name != "Read" && sel.Sel.Name != "Write" {
		return false
	}
	if pass != nil && pass.TypesInfo != nil {
		t := pass.TypesInfo.TypeOf(sel.X)
		return t != nil && strings.Contains(t.String(), "net.Conn")
	}
	return false
}

func isExecCmdCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	switch sel.Sel.Name {
	case "Run", "Output", "CombinedOutput", "Start":
		if pass != nil && pass.TypesInfo != nil {
			t := pass.TypesInfo.TypeOf(sel.X)
			return t != nil && strings.Contains(t.String(), "os/exec.Cmd")
		}
	}
	return false
}

func isOSFileCall(pass *analysis.Pass, sel *ast.SelectorExpr) bool {
	switch sel.Sel.Name {
	case "Read", "Write", "ReadAt", "WriteAt", "WriteString", "Sync":
		if pass != nil && pass.TypesInfo != nil {
			t := pass.TypesInfo.TypeOf(sel.X)
			return t != nil && strings.Contains(t.String(), "os.File")
		}
	}
	return false
}

func unwrapPointer(t types.Type) types.Type {
	for {
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		} else {
			break
		}
	}
	return t
}
