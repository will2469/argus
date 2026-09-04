package a08_tx_io

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
		"./tests/correctness/a08/positive",
		"./tests/correctness/a08/negative",
	)
}

func TestCheckBlockingIO(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"net/http"
	"os"
	"os/exec"
	"time"
)

type Calculator struct{}
func (c *Calculator) Upload(val int) {}

type StorageClient struct{}
func (s *StorageClient) Upload(data []byte) {}

func foo(ch chan int, calc *Calculator, storage *StorageClient) {
	time.Sleep(1)
	http.Get("http://example.com")
	os.ReadFile("file.txt")
	exec.Command("ls")
	storage.Upload([]byte("data"))

	// Non-blocking external I/O: in-memory synchronization & non-storage upload
	ch <- 1
	<-ch
	calc.Upload(123)
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var ops []string
	ast.Inspect(file, func(n ast.Node) bool {
		if op := CheckBlockingIO(n, nil); op != "" {
			ops = append(ops, op)
		}
		return true
	})

	if len(ops) != 5 {
		t.Errorf("expected exactly 5 blocking external I/O ops (sleep, http, read, exec, storage), got %d: %v", len(ops), ops)
	}
}
