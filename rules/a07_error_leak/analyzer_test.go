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
	"encoding/json"
	"fmt"
	"net/http"
)
func handler(w http.ResponseWriter) {
	http.Error(w, "err", 500)
	w.Write([]byte("err"))
	json.NewEncoder(w).Encode("err")
	fmt.Fprintf(w, "%s", "err")
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var sinksCount int
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			sink := InspectResponseSink(call)
			if sink.IsSink {
				sinksCount++
			}
		}
		return true
	})

	if sinksCount != 4 {
		t.Errorf("expected 4 sinks, got %d", sinksCount)
	}
}
