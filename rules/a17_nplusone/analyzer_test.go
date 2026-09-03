package a17_nplusone

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve rootDir: %v", err)
	}

	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a17/positive",
		"./tests/correctness/a17/negative",
	)
}

func TestTransitiveCallGraphDetection(t *testing.T) {
	src := `package testpkg

type DB struct{}
func (DB) Query(sql string) {}

func level1(db DB) { db.Query("SELECT 1") }
func level2(db DB) { level1(db) }
func level3(db DB) { level2(db) }
func level4(db DB) { level3(db) }
func unrelated() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	detector := NewHelperQueryDetector(nil, file)

	helpers := []string{"level1", "level2", "level3", "level4"}
	for _, h := range helpers {
		if !detector.funcHasQuery[h] {
			t.Errorf("expected %s to be marked as query helper across transitive calls", h)
		}
	}
	if detector.funcHasQuery["unrelated"] {
		t.Errorf("unrelated function must not be marked as query helper")
	}
}

func TestIsDBQueryCall_Rejection(t *testing.T) {
	makeCall := func(recvName, methodName string) *ast.CallExpr {
		return &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.Ident{Name: recvName},
				Sel: &ast.Ident{Name: methodName},
			},
			Args: []ast.Expr{
				&ast.Ident{Name: "ctx"},
				&ast.BasicLit{Kind: token.STRING, Value: `"query_str"`},
			},
		}
	}

	nonDBCalls := []string{"search", "searchengine", "httpClient", "metrics", "solr", "client", "req"}
	for _, name := range nonDBCalls {
		call := makeCall(name, "Query")
		if IsDBQueryCall(nil, call) {
			t.Errorf("call on %s.Query must NOT be identified as a DB query", name)
		}
	}

	dbCall := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "db"},
			Sel: &ast.Ident{Name: "Query"},
		},
		Args: []ast.Expr{
			&ast.Ident{Name: "ctx"},
			&ast.BasicLit{Kind: token.STRING, Value: `"SELECT 1"`},
		},
	}
	if !IsDBQueryCall(nil, dbCall) {
		t.Errorf("call on db.Query must be identified as a DB query")
	}
}

func parseWithTypes(t *testing.T, filenames []string, sources []string) (*analysis.Pass, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	for i, src := range sources {
		f, err := parser.ParseFile(fset, filenames[i], src, parser.ParseComments)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", filenames[i], err)
		}
		files = append(files, f)
	}

	typesInfo := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	conf := types.Config{
		Error: func(err error) {},
	}
	pkg, _ := conf.Check(files[0].Name.Name, fset, files, typesInfo)
	pass := &analysis.Pass{
		Fset:      fset,
		Files:     files,
		Pkg:       pkg,
		TypesInfo: typesInfo,
	}
	return pass, fset
}

func TestFunctionIdentity_ReceiverCollision(t *testing.T) {
	srcA := `package repo_collision

type Item struct{ ID int }
type DB struct{}
func (DB) Query(sql string, id int) {}
func (DB) Exec(sql string, args ...any) {}

type DBRepo struct{ db DB }
func (r DBRepo) Get(id int) {
	r.db.Query("SELECT * FROM items WHERE id = $1", id)
}

type MemoryCache struct{}
func (MemoryCache) Get(id int) string {
	return "cached"
}

func handlerCache(cache MemoryCache, items []Item) {
	for _, item := range items {
		cache.Get(item.ID)
	}
}

func handlerRepo(repo DBRepo, items []Item) {
	for _, item := range items {
		repo.Get(item.ID)
	}
}
`
	passA, fsetA := parseWithTypes(t, []string{"repo_a.go"}, []string{srcA})
	detectorA := NewHelperQueryDetector(passA, passA.Files...)
	issuesA := WalkLoops(passA, fsetA, passA.Files[0], nil, detectorA)
	if len(issuesA) != 1 {
		t.Fatalf("expected exactly 1 issue for handlerRepo, got %d", len(issuesA))
	}
	if !strings.Contains(issuesA[0].Message, "DBRepo") {
		t.Errorf("expected issue message to mention DBRepo, got: %s", issuesA[0].Message)
	}

	// Declaration order invariance: MemoryCache declared before DBRepo
	srcB := `package repo_collision

type Item struct{ ID int }
type DB struct{}
func (DB) Query(sql string, id int) {}
func (DB) Exec(sql string, args ...any) {}

type MemoryCache struct{}
func (MemoryCache) Get(id int) string {
	return "cached"
}

type DBRepo struct{ db DB }
func (r DBRepo) Get(id int) {
	r.db.Query("SELECT * FROM items WHERE id = $1", id)
}

func handlerCache(cache MemoryCache, items []Item) {
	for _, item := range items {
		cache.Get(item.ID)
	}
}

func handlerRepo(repo DBRepo, items []Item) {
	for _, item := range items {
		repo.Get(item.ID)
	}
}
`
	passB, fsetB := parseWithTypes(t, []string{"repo_b.go"}, []string{srcB})
	detectorB := NewHelperQueryDetector(passB, passB.Files...)
	issuesB := WalkLoops(passB, fsetB, passB.Files[0], nil, detectorB)
	if len(issuesB) != 1 {
		t.Fatalf("expected exactly 1 issue for handlerRepo (order invariance), got %d", len(issuesB))
	}
	if !strings.Contains(issuesB[0].Message, "DBRepo") {
		t.Errorf("expected issue message to mention DBRepo, got: %s", issuesB[0].Message)
	}
}

func TestSemanticHelperWithoutNamingFilter(t *testing.T) {
	src := `package semantic_test

type User struct{ ID int }
type DB struct{}
func (DB) Query(sql string, id int) {}
func (DB) Exec(sql string, args ...any) {}

var db DB

func hydrateUser(id int) {
	db.Query("SELECT * FROM users WHERE id = $1", id)
}

func syncRecord(id int) {
	db.Query("UPDATE records SET synced = true WHERE id = $1", id)
}

func handler(users []User) {
	for _, u := range users {
		hydrateUser(u.ID)
	}
	for _, u := range users {
		syncRecord(u.ID)
	}
}
`
	pass, fset := parseWithTypes(t, []string{"semantic.go"}, []string{src})
	detector := NewHelperQueryDetector(pass, pass.Files...)
	issues := WalkLoops(pass, fset, pass.Files[0], nil, detector)

	if len(issues) != 2 {
		t.Fatalf("expected 2 issues for hydrateUser and syncRecord without prefix filters, got %d", len(issues))
	}
}

func TestPackageWideCrossFileCallGraph(t *testing.T) {
	repoSrc := `package service

type UserRepo struct{}
type DB struct{}
func (DB) Query(sql string, id int) {}
func (DB) Exec(sql string, args ...any) {}

var db DB

func (UserRepo) GetUser(id int) {
	db.Query("SELECT * FROM users WHERE id = $1", id)
}
`
	handlerSrc := `package service

type User struct{ ID int }

func ProcessUsers(repo UserRepo, users []User) {
	for _, u := range users {
		repo.GetUser(u.ID)
	}
}
`
	pass, fset := parseWithTypes(t, []string{"repository.go", "handler.go"}, []string{repoSrc, handlerSrc})
	detector := NewHelperQueryDetector(pass, pass.Files...)
	handlerFile := pass.Files[1]
	issues := WalkLoops(pass, fset, handlerFile, nil, detector)

	if len(issues) != 1 {
		t.Fatalf("expected 1 cross-file N+1 issue in handler.go, got %d", len(issues))
	}
	if !strings.Contains(issues[0].Message, "GetUser") {
		t.Errorf("expected issue message to identify GetUser, got: %s", issues[0].Message)
	}
}
