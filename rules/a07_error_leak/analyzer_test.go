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
			sink := InspectResponseSink(nil, file, call, fn)
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

func TestErrorProvenanceSoundness(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"database/sql"
	"errors"
	"net/http"
)

type CustomService struct{}
func (s *CustomService) Ping() error { return errors.New("service ping failed") }
func (s *CustomService) Err() error { return errors.New("custom err") }
func (s *CustomService) Close() error { return errors.New("closed") }

type MemoryStore struct{}
func (m *MemoryStore) Get(k string) (string, error) { return "", errors.New("missing") }

// 1. Unrelated error named sqlErr must NOT be flagged as DB error leak
func nonDBNamedError(w http.ResponseWriter) {
	sqlErr := errors.New("totally unrelated")
	http.Error(w, sqlErr.Error(), 400)
}

// 2. Ping/Err/Close on custom non-DB service must NOT be flagged
func nonDBMethods(w http.ResponseWriter, s *CustomService) {
	err := s.Ping()
	if err != nil {
		http.Error(w, err.Error(), 502)
	}
}

// 3. Receiver named store on in-memory store must NOT be flagged
func memoryStore(w http.ResponseWriter, store *MemoryStore) {
	_, err := store.Get("foo")
	if err != nil {
		http.Error(w, err.Error(), 404)
	}
}

// 4. Multi-value DB query must be FLAGGED
func dbQuery(w http.ResponseWriter, db *sql.DB) {
	rows, err := db.Query("SELECT 1")
	_ = rows
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}

// 5. DB Ping on actual *sql.DB must be FLAGGED
func dbPing(w http.ResponseWriter, db *sql.DB) {
	err := db.Ping()
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}
`
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	issues := InspectFile(nil, fset, file, nil)

	// We expect exactly 2 issues: from dbQuery (line 44) and dbPing (line 52)
	if len(issues) != 2 {
		t.Fatalf("expected exactly 2 issues from actual DB calls, got %d", len(issues))
	}

	for _, iss := range issues {
		pos := fset.Position(iss.Pos)
		t.Logf("Detected expected DB issue at line %d: %s", pos.Line, iss.Message)
	}
}

func TestPackageShadowingAndAuxiliaryProof(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main
import (
	"bufio"
	"io"
	"net/http"
	stdErr "errors"
)

type FakeErrors struct{}
func (FakeErrors) New(s string) error { return stdErr.New(s) }

// 1. bufio.Scanner.Err() must NOT be flagged as DB error
func scannerErr(w http.ResponseWriter, r io.Reader) {
	sc := bufio.NewScanner(r)
	if err := sc.Err(); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

// 2. io.Closer.Close() must NOT be flagged as DB error
func closerErr(w http.ResponseWriter, c io.Closer) {
	if err := c.Close(); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

// 3. Aliased import stdErr.New produces clean error
func aliasedImport(w http.ResponseWriter) {
	err := stdErr.New("aliased clean error")
	http.Error(w, err.Error(), 400)
}
`
	file, err := parser.ParseFile(fset, "test2.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		for _, iss := range issues {
			pos := fset.Position(iss.Pos)
			t.Errorf("unexpected issue on non-DB / clean error at Line %d: %s", pos.Line, iss.Message)
		}
	}
}

