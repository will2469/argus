package a05_audit_immutability

import (
	"testing"
)

func TestVariableShadowing_InnerMutatesOuterSafe(t *testing.T) {
	src := `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}
func ShadowTest(ctx context.Context, db DB) {
	query := "SELECT * FROM audit_logs"
	{
		query := "UPDATE audit_logs SET action = 'HACK' WHERE id = 1"
		db.Exec(ctx, query)
	}
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil, map[string]bool{"audit_logs": true})
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue for inner shadowed UPDATE, got %d: %+v", len(issues), issues)
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 10 {
		t.Errorf("expected issue at line 10, got line %d", pos.Line)
	}
}

func TestVariableShadowing_OuterMutatesInnerSafe(t *testing.T) {
	src := `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}
func ShadowTest(ctx context.Context, db DB) {
	query := "UPDATE audit_logs SET action = 'HACK' WHERE id = 1"
	{
		query := "SELECT * FROM audit_logs"
		db.Exec(ctx, query)
	}
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil, map[string]bool{"audit_logs": true})
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue for outer UPDATE, got %d: %+v", len(issues), issues)
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 12 {
		t.Errorf("expected issue at line 12, got line %d", pos.Line)
	}
}

func TestPathSensitivity_BranchReassignment(t *testing.T) {
	// User's exact case: initial safe query, branch assigns UPDATE.
	src := `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}
func BranchTest(ctx context.Context, db DB, cond bool) {
	query := "SELECT * FROM audit_logs"
	if cond {
		query = "UPDATE audit_logs SET action = 'MUTATED' WHERE id = 1"
	}
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil, map[string]bool{"audit_logs": true})
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for conditional UPDATE, got %d: %+v", len(issues), issues)
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 11 {
		t.Errorf("expected issue at line 11, got line %d", pos.Line)
	}
}

func TestPathSensitivity_InitialUnsafeBranchSafe(t *testing.T) {
	// Initial UPDATE, branch assigns SELECT. If cond is false, UPDATE executes!
	src := `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}
func BranchTest(ctx context.Context, db DB, cond bool) {
	query := "UPDATE audit_logs SET action = 'MUTATED' WHERE id = 1"
	if cond {
		query = "SELECT * FROM audit_logs"
	}
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil, map[string]bool{"audit_logs": true})
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue when bypass path executes UPDATE, got %d: %+v", len(issues), issues)
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 11 {
		t.Errorf("expected issue at line 11, got line %d", pos.Line)
	}
}

func TestPathSensitivity_IfElseBothBranches(t *testing.T) {
	src := `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}
func IfElseTest(ctx context.Context, db DB, cond bool) {
	var query string
	if cond {
		query = "UPDATE audit_logs SET action = 'MUTATED' WHERE id = 1"
	} else {
		query = "SELECT * FROM audit_logs"
	}
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil, map[string]bool{"audit_logs": true})
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for if-else branch with UPDATE, got %d: %+v", len(issues), issues)
	}
}

func TestSequentialReassignment_CleanOverride(t *testing.T) {
	// Unconditional sequential reassignment kills earlier definition
	src := `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}
func CleanOverride(ctx context.Context, db DB) {
	query := "UPDATE audit_logs SET action = 'MUTATED' WHERE id = 1"
	query = "SELECT * FROM audit_logs"
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil, map[string]bool{"audit_logs": true})
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when UPDATE is killed by sequential reassignment, got %d: %+v", len(issues), issues)
	}
}

func TestSequentialReassignment_DirtyOverride(t *testing.T) {
	// Unconditional sequential reassignment overwrites safe with unsafe
	src := `package main
import "context"
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (any, error)
}
func DirtyOverride(ctx context.Context, db DB) {
	query := "SELECT * FROM audit_logs"
	query = "UPDATE audit_logs SET action = 'MUTATED' WHERE id = 1"
	db.Exec(ctx, query)
}`
	fset, file := parseSnippet(t, src)
	issues := InspectFile(nil, fset, file, nil, map[string]bool{"audit_logs": true})
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue when query is dirtied, got %d: %+v", len(issues), issues)
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 9 {
		t.Errorf("expected issue at line 9, got line %d", pos.Line)
	}
}
