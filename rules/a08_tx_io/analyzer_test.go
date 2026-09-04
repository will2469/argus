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
		if op := CheckBlockingIOWithContext(n, nil, nil, file); op != "" {
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

func TestCalculatorUploadInTx_MustBeSafe(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"database/sql"
)

type Calculator struct{}
func (c *Calculator) Upload(val int) {}

func process(db *sql.DB, calculator *Calculator) {
	tx, _ := db.Begin()
	if true {
		calculator.Upload(100)
	}
	tx.Commit()
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for calculator.Upload inside transaction, got %d: %v", len(issues), issues)
	}
}

func TestClientPutObjectInTx_MustBeFlagged(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"database/sql"
)

type S3Client interface {
	PutObject(key string, data []byte) error
}

func process(db *sql.DB, client S3Client) {
	tx, _ := db.Begin()
	_ = client.PutObject("key", []byte("data"))
	tx.Commit()
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for client.PutObject inside transaction, got %d: %v", len(issues), issues)
	}
}

func TestMethodUploadDoesNotTriggerPackageUploadHelper(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"database/sql"
	"time"
)

type Calculator struct{}
func (c *Calculator) Upload(val int) {}

func Upload() {
	time.Sleep(1)
}

func process(db *sql.DB, calculator *Calculator) {
	tx, _ := db.Begin()
	calculator.Upload(100)
	tx.Commit()
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when calling method calculator.Upload even if package func Upload has Sleep, got %d: %v", len(issues), issues)
	}
}

func TestStoreNamedVariableWithCalculator_MustBeSafe(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"database/sql"
)

type Calculator struct{}
func (c *Calculator) Upload(val int) {}

func process(db *sql.DB) {
	tx, _ := db.Begin()
	store := &Calculator{}
	store.Upload(100)
	tx.Commit()
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for variable named store of type Calculator, got %d: %v", len(issues), issues)
	}
}


