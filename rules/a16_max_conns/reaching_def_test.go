package a16_max_conns

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestDSNReachingDefinitions_SequentialOverride(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context) {
	dsn := "postgres://bad/db"
	dsn = "postgres://good/db?pool_max_conns=20"
	pgxpool.New(ctx, dsn)
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when safe DSN overrides unsafe DSN, got %d: %v", len(issues), issues[0].Message)
	}
}

func TestDSNReachingDefinitions_BranchOverride(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context, prod bool) {
	var dsn string
	if prod {
		dsn = "postgres://bad/db"
	}
	dsn = "postgres://good/db?pool_max_conns=20"
	pgxpool.New(ctx, dsn)
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when post-branch safe DSN kills branch DSN, got %d: %v", len(issues), issues[0].Message)
	}
}

func TestDSNReachingDefinitions_UnsafeBranchKept(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context, prod bool) {
	var dsn string
	if prod {
		dsn = "postgres://insecure/db"
	} else {
		dsn = "postgres://secure/db?pool_max_conns=20"
	}
	pgxpool.New(ctx, dsn)
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue when unsafe branch reaches call, got %d", len(issues))
	}
}

func TestDSNReachingDefinitions_DeclOverride(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context) {
	var dsn = "postgres://bad/db"
	dsn = "postgres://good/db?pool_max_conns=20"
	pgxpool.New(ctx, dsn)
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when safe DSN overrides initial var decl, got %d", len(issues))
	}
}

func TestDSNReachingDefinitions_SwitchOverride(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context, mode int) {
	var dsn string
	switch mode {
	case 1:
		dsn = "postgres://bad/db"
	default:
		dsn = "postgres://insecure/db"
	}
	dsn = "postgres://good/db?pool_max_conns=20"
	pgxpool.New(ctx, dsn)
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when safe DSN kills switch branch definitions, got %d", len(issues))
	}
}

func TestDSNReachingDefinitions_SwitchUnsafeKept(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context, mode int) {
	var dsn string
	switch mode {
	case 1:
		dsn = "postgres://bad/db"
	default:
		dsn = "postgres://good/db?pool_max_conns=20"
	}
	pgxpool.New(ctx, dsn)
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue when unsafe switch branch reaches call, got %d", len(issues))
	}
}

func TestDSNReachingDefinitions_SelfConcat(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context) {
	dsn := "postgres://bad/db"
	dsn = dsn + "?pool_max_conns=20"
	pgxpool.New(ctx, dsn)
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when DSN string concatenation appends pool_max_conns, got %d: %v", len(issues), issues)
	}
}

func TestDSNReachingDefinitions_AssignRhsCall(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) (any, error) { return nil, nil }
func Run(ctx context.Context) {
	dsn := "postgres://bad/db"
	pool, err := pgxpool.New(ctx, dsn)
	_ = pool
	_ = err
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue when unsafe DSN is read in RHS call of assignment, got %d", len(issues))
	}
}
