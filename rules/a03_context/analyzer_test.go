package a03_context

import (
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
		"./tests/correctness/a03/positive",
		"./tests/correctness/a03/negative",
	)
}

func parseHelper(t *testing.T, src string) []Issue {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return InspectFile(nil, fset, file, nil)
}

func TestInspectFile_ImportAlias(t *testing.T) {
	src := `package testpkg
import stdctx "context"

type DB interface {
	Query(ctx stdctx.Context, sql string, args ...any) (any, error)
}

func TestAlias(db DB) {
	ctx := stdctx.Background()
	_, _ = db.Query(ctx, "SELECT 1")
}
`
	issues := parseHelper(t, src)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for aliased stdctx.Background(), got %d", len(issues))
	}
}

func TestInspectFile_ShadowingClean(t *testing.T) {
	src := `package testpkg
import (
	"context"
	"time"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func TestShadow(db DB) {
	ctx := context.Background()
	if true {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, _ = db.Query(ctx, "SELECT 1")
	}
	_ = ctx
}
`
	issues := parseHelper(t, src)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when inner shadowed ctx is bounded, got %d", len(issues))
	}
}

func TestInspectFile_ShadowingRaw(t *testing.T) {
	src := `package testpkg
import (
	"context"
	"time"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func TestShadowRaw(db DB) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if true {
		ctx := context.Background()
		_, _ = db.Query(ctx, "SELECT 1")
	}
	_ = ctx
}
`
	issues := parseHelper(t, src)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue when inner shadowed ctx is raw Background, got %d", len(issues))
	}
}

func TestInspectFile_BranchIncomplete(t *testing.T) {
	src := `package testpkg
import (
	"context"
	"time"
)

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func TestBranch(db DB, cond bool) {
	ctx := context.Background()
	if cond {
		ctx, _ = context.WithTimeout(context.Background(), time.Second)
	}
	_, _ = db.Query(ctx, "SELECT 1")
}
`
	issues := parseHelper(t, src)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue when branch leaves context raw on false path, got %d", len(issues))
	}
}

func TestInspectFile_WithCancelAllowed(t *testing.T) {
	src := `package testpkg
import "context"

type DB interface {
	Query(ctx context.Context, sql string, args ...any) (any, error)
}

func TestCancel(db DB) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, _ = db.Query(ctx, "SELECT 1")
}
`
	issues := parseHelper(t, src)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for context.WithCancel, got %d", len(issues))
	}
}
