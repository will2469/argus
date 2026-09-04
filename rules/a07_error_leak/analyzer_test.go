package a07_error_leak

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a07/positive",
		"./tests/correctness/a07/negative",
	)
}

func TestInspectResponseSink(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)
func handler(w http.ResponseWriter) {
	http.Error(w, "err", 500)
	w.Write([]byte("err"))
	json.NewEncoder(w).Encode("err")
	fmt.Fprintf(w, "%s", "err")

	// Non-sinks: local memory buffer operations
	var buf bytes.Buffer
	buf.Write([]byte("err"))
	json.NewEncoder(&buf).Encode("err")
	fmt.Fprintf(&buf, "%s", "err")
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == "handler" {
			fn = f
			break
		}
	}

	var sinksCount int
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			sink := InspectResponseSink(nil, call, fn)
			if sink.IsSink {
				sinksCount++
			}
		}
		return true
	})

	if sinksCount != 4 {
		t.Errorf("expected exactly 4 response sinks (w.*), got %d (buffers falsely identified as sinks)", sinksCount)
	}
}
