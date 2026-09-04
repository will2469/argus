package a16_max_conns

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestDSNReachingDefinitions_FallthroughChainSafe(t *testing.T) {
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
		fallthrough
	case 2:
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
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when case 2 overrides fallthrough definition, got %d: %v", len(issues), issues)
	}
}

func TestDSNReachingDefinitions_FallthroughChainUnsafe(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context, mode int) {
	var dsn string
	switch mode {
	case 1:
		dsn = "postgres://good/db?pool_max_conns=20"
		fallthrough
	case 2:
		dsn = "postgres://bad/db"
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
		t.Fatalf("expected 1 issue when fallthrough leads to unsafe definition, got %d", len(issues))
	}
}

func TestDSNReachingDefinitions_FallthroughChainNoOverride(t *testing.T) {
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
		fallthrough
	case 2:
		// no reassignment, bad falls through
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
		t.Fatalf("expected 1 issue when fallthrough propagates unoverridden bad DSN, got %d", len(issues))
	}
}

func TestDSNReachingDefinitions_ShadowedScope(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func use(s string) {}
func Run(ctx context.Context, prod bool) {
	dsn := "postgres://good/db?pool_max_conns=20"
	if prod {
		dsn := "postgres://bad/db"
		use(dsn)
	}
	pgxpool.New(ctx, dsn)
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when inner shadowed := does not mutate outer variable, got %d: %v", len(issues), issues)
	}
}

func TestDSNReachingDefinitions_NonShadowedMutation(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context, prod bool) {
	dsn := "postgres://good/db?pool_max_conns=20"
	if prod {
		dsn = "postgres://bad/db"
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
		t.Fatalf("expected 1 issue when non-shadowed assignment mutates outer variable, got %d", len(issues))
	}
}

func TestDSNReachingDefinitions_AllTerminatingSwitch(t *testing.T) {
	src := `package main
import "context"
type pgxpoolPkg struct{}
var pgxpool pgxpoolPkg
func (pgxpoolPkg) New(ctx context.Context, dsn string) {}
func Run(ctx context.Context, mode int) {
	dsn := "postgres://good/db?pool_max_conns=20"
	switch mode {
	case 1:
		panic("fail")
	default:
		panic("other")
	}
	pgxpool.New(ctx, dsn)
}`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	issues := InspectFile(nil, fset, file, nil, 100)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when all-terminating switch preserves inSet, got %d", len(issues))
	}
}
