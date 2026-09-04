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

func TestRejectFakeStore(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import "time"

type FakeStore struct{}
func (FakeStore) Begin() *FakeTx { return &FakeTx{} }
type FakeTx struct{}
func (FakeTx) Foo() {}

func process(fakeStore FakeStore) {
	tx := fakeStore.Begin()
	time.Sleep(1)
	_ = tx
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var issues []Issue
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			InspectExplicitTxFlow(nil, fset, fn, file, nil, nil, nil, &issues)
		}
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for FakeStore, got %d: %v", len(issues), issues)
	}
}

func TestTxFlowNestedIf(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"database/sql"
	"time"
)

func process(db *sql.DB) {
	tx, _ := db.Begin()
	if true {
		time.Sleep(1)
	}
	tx.Commit()
}

func processSafe(db *sql.DB) {
	tx, _ := db.Begin()
	tx.Commit()
	if true {
		time.Sleep(1)
	}
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var issues []Issue
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "process" {
			InspectExplicitTxFlow(nil, fset, fn, file, nil, nil, nil, &issues)
		}
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for I/O inside nested if of active transaction, got %d", len(issues))
	}

	var safeIssues []Issue
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "processSafe" {
			InspectExplicitTxFlow(nil, fset, fn, file, nil, nil, nil, &safeIssues)
		}
	}
	if len(safeIssues) != 0 {
		t.Fatalf("expected 0 issues for I/O after Commit, got %d", len(safeIssues))
	}
}

