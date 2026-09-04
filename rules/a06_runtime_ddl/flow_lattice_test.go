package a06_runtime_ddl

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func parseSnippet(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse snippet: %v", err)
	}
	return fset, file
}

func TestLattice_BranchingConditionalDDL(t *testing.T) {
	src := `package main
import "database/sql"
type DB interface {
	Exec(ctx any, sql string, args ...any) (sql.Result, error)
}
func BranchTest(ctx any, db DB, cond bool) {
	query := "SELECT 1"
	if cond {
		query = "CREATE TABLE users (id int)"
	}
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for conditional DDL (MAYBE_DDL), got %d: %+v", len(issues), issues)
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 11 {
		t.Errorf("expected issue at line 11, got line %d", pos.Line)
	}
}

func TestLattice_BranchingConditionalClean(t *testing.T) {
	src := `package main
import "database/sql"
type DB interface {
	Exec(ctx any, sql string, args ...any) (sql.Result, error)
}
func BranchTest(ctx any, db DB, cond bool) {
	query := "CREATE TABLE users (id int)"
	if cond {
		query = "SELECT 1"
	}
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue when bypass path executes DDL, got %d: %+v", len(issues), issues)
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 11 {
		t.Errorf("expected issue at line 11, got line %d", pos.Line)
	}
}

func TestLattice_SequentialCleanOverride(t *testing.T) {
	src := `package main
import "database/sql"
type DB interface {
	Exec(ctx any, sql string, args ...any) (sql.Result, error)
}
func CleanOverride(ctx any, db DB) {
	query := "CREATE TABLE users (id int)"
	query = "SELECT 1"
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when DDL is killed by sequential clean override, got %d: %+v", len(issues), issues)
	}
}

func TestLattice_VariableShadowing_InnerDDLOuterSafe(t *testing.T) {
	src := `package main
import "database/sql"
type DB interface {
	Exec(ctx any, sql string, args ...any) (sql.Result, error)
}
func ShadowTest(ctx any, db DB) {
	query := "SELECT 1"
	{
		query := "CREATE TABLE users (id int)"
		db.Exec(ctx, query)
	}
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue for inner shadowed DDL, got %d: %+v", len(issues), issues)
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 10 {
		t.Errorf("expected issue at line 10, got line %d", pos.Line)
	}
}

func TestLattice_VariableShadowing_OuterDDLInnerSafe(t *testing.T) {
	src := `package main
import "database/sql"
type DB interface {
	Exec(ctx any, sql string, args ...any) (sql.Result, error)
}
func ShadowTest(ctx any, db DB) {
	query := "CREATE TABLE users (id int)"
	{
		query := "SELECT 1"
		db.Exec(ctx, query)
	}
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue for outer DDL, got %d: %+v", len(issues), issues)
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 12 {
		t.Errorf("expected issue at line 12, got line %d", pos.Line)
	}
}

func TestLattice_SemanticBuilder_NonBuilderIgnored(t *testing.T) {
	src := `package main
import "database/sql"
type CustomLogger struct{}
func (c *CustomLogger) WriteString(s string) {}
func (c *CustomLogger) Reset() {}
type DB interface {
	Exec(ctx any, sql string, args ...any) (sql.Result, error)
}
func NonBuilderTest(ctx any, db DB, logger *CustomLogger) {
	logger.WriteString("CREATE TABLE evil (id int)")
	db.Exec(ctx, "SELECT 1")
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when WriteString is on custom non-builder type, got %d: %+v", len(issues), issues)
	}
}
