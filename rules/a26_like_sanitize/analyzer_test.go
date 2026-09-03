package a26_like_sanitize

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
		t.Fatalf("failed to resolve rootDir: %v", err)
	}

	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a26/positive",
		"./tests/correctness/a26/negative",
	)
}

func TestFindLikeParamIndices(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected []int
	}{
		{
			name:     "single like param",
			sql:      "SELECT id FROM users WHERE name ILIKE $1",
			expected: []int{1},
		},
		{
			name:     "multiple params, like is second",
			sql:      "SELECT id FROM users WHERE status = $1 AND name LIKE $2",
			expected: []int{2},
		},
		{
			name:     "concatenated like pattern in sql",
			sql:      "SELECT id FROM users WHERE name ILIKE '%' || $1 || '%'",
			expected: []int{1},
		},
		{
			name:     "no like clause",
			sql:      "SELECT id FROM users WHERE id = $1",
			expected: nil,
		},
		{
			name:     "fallback fragment with concat function",
			sql:      "WHERE name LIKE CONCAT('%', $1, '%')",
			expected: []int{1},
		},
		{
			name:     "fallback fragment with pipe concatenation",
			sql:      "WHERE name LIKE '%' || $1 || '%'",
			expected: []int{1},
		},
		{
			name:     "nested lower function in like",
			sql:      "WHERE name LIKE LOWER($1)",
			expected: []int{1},
		},
		{
			name:     "multiple clauses stops like pattern at AND",
			sql:      "WHERE name LIKE $1 AND status = $2",
			expected: []int{1},
		},
		{
			name:     "enclosed parenthesis where clause",
			sql:      "WHERE (name LIKE '%' || $1 || '%')",
			expected: []int{1},
		},
		{
			name:     "string literal containing AND inside pattern",
			sql:      "WHERE name LIKE '% AND %' || $1",
			expected: []int{1},
		},
		{
			name:     "multiple like clauses with or",
			sql:      "WHERE name ILIKE $1 OR email ILIKE $2",
			expected: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindLikeParamIndices(tt.sql)
			if len(got) != len(tt.expected) {
				t.Fatalf("FindLikeParamIndices() = %v, want %v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("FindLikeParamIndices()[%d] = %d, want %d", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestFindLikeParamIndicesRegex_ScannerDirect(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected []int
	}{
		{
			name:     "concat function in regex fallback",
			sql:      "WHERE name LIKE CONCAT('%', $1, '%')",
			expected: []int{1},
		},
		{
			name:     "pipe concatenation in regex fallback",
			sql:      "WHERE name LIKE '%' || $1 || '%'",
			expected: []int{1},
		},
		{
			name:     "stops at boundary AND in regex fallback",
			sql:      "WHERE name LIKE $1 AND id = $2",
			expected: []int{1},
		},
		{
			name:     "literal AND inside quotes does not stop scanner",
			sql:      "WHERE name LIKE '% AND %' || $1",
			expected: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findLikeParamIndicesRegex(tt.sql)
			if len(got) != len(tt.expected) {
				t.Fatalf("findLikeParamIndicesRegex() = %v, want %v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("findLikeParamIndicesRegex()[%d] = %d, want %d", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestIsArgumentSanitized_SemanticWhitelist(t *testing.T) {
	fset := token.NewFileSet()
	src := `package main

import "strings"

type Evil struct{}
func (Evil) SanitizeLikePattern(s string) string { return s }

func SanitizeLike(s string) string {
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// argus:trusted-sanitizer custom assembly escape engine
func FastSanitize(s string) string {
	return s
}
`
	file, err := parser.ParseFile(fset, "san.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	reg := NewSanitizerRegistry(nil, []*ast.File{file}, nil, nil)

	// Call expression AST helpers
	makeIdentCall := func(name string) *ast.CallExpr {
		return &ast.CallExpr{Fun: &ast.Ident{Name: name}}
	}
	makeSelCall := func(x, sel string) *ast.CallExpr {
		return &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   &ast.Ident{Name: x},
				Sel: &ast.Ident{Name: sel},
			},
		}
	}

	// 1. Unverified ReplaceAll is rejected
	if reg.IsSanitizerCall(nil, makeIdentCall("ReplaceAll")) {
		t.Errorf("ReplaceAll must NOT be considered a safe LIKE sanitizer")
	}

	// 2. Fake evil.SanitizeLikePattern is REJECTED even if method name matches
	if reg.IsSanitizerCall(nil, makeSelCall("evil", "SanitizeLikePattern")) {
		t.Errorf("evil.SanitizeLikePattern must NOT be accepted without verified escaping")
	}

	// 3. Statically verified local SanitizeLike is ACCEPTED
	if !reg.IsSanitizerCall(nil, makeIdentCall("SanitizeLike")) {
		t.Errorf("SanitizeLike must be accepted via AST escaping verification")
	}

	// 4. Annotated // argus:trusted-sanitizer is ACCEPTED
	if !reg.IsSanitizerCall(nil, makeIdentCall("FastSanitize")) {
		t.Errorf("FastSanitize must be accepted via trusted-sanitizer directive")
	}
}

func TestFlowSensitiveDataflow(t *testing.T) {
	src := `package main

import "strings"

func SanitizeLike(s string) string {
	s = strings.ReplaceAll(s, "%", ` + "`\\%`" + `)
	s = strings.ReplaceAll(s, "_", ` + "`\\_`" + `)
	return s
}

func FormatLikeContains(s string) string {
	return "%" + SanitizeLike(s) + "%"
}

func fn(userInput string, trusted bool) {
	// 1. Overwrite: should be UNSAFE
	pattern1 := userInput
	pattern1 = SanitizeLike(pattern1)
	pattern1 = userInput
	query(pattern1)

	// 2. Conditional no else: should be UNSAFE
	pattern2 := userInput
	if trusted {
		pattern2 = SanitizeLike(pattern2)
	}
	query(pattern2)

	// 3. Conditional both branches: should be SAFE
	pattern3 := userInput
	if trusted {
		pattern3 = SanitizeLike(pattern3)
	} else {
		pattern3 = FormatLikeContains(pattern3)
	}
	query(pattern3)

	// 4. Assign after query: should be UNSAFE
	pattern4 := userInput
	query(pattern4)
	pattern4 = SanitizeLike(pattern4)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	reg := NewSanitizerRegistry(nil, []*ast.File{f}, nil, nil)
	fn := f.Decls[2].(*ast.FuncDecl)
	expected := map[string]bool{
		"pattern1": false,
		"pattern2": false,
		"pattern3": true,
		"pattern4": false,
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || id.Name != "query" {
			return true
		}
		argIdent := call.Args[0].(*ast.Ident)
		wantSafe, tracked := expected[argIdent.Name]
		if !tracked {
			return true
		}
		gotSafe := isIdentSanitized(argIdent, fn.Body, reg, nil)
		if gotSafe != wantSafe {
			t.Errorf("variable %s: expected safe=%v, got safe=%v", argIdent.Name, wantSafe, gotSafe)
		}
		return true
	})
}

func TestPathologicalLiterals(t *testing.T) {
	reg := NewSanitizerRegistry(nil, nil, nil, nil)

	makeLit := func(v string) *ast.BasicLit {
		return &ast.BasicLit{Kind: token.STRING, Value: `"` + v + `"`}
	}

	pathological := []string{
		"%",
		"%%",
		"%%%",
		"%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%",
		"_%_",
	}
	for _, p := range pathological {
		isPatho, _ := reg.IsPathologicalLiteral(makeLit(p))
		if !isPatho {
			t.Errorf("expected %q to be flagged as pathological literal", p)
		}
	}

	safe := []string{
		"PENDING_%",
		"STATUS_%",
		"ORDER-2024-%",
		"user_name",
		"PREFIX_%_SUFFIX",
	}
	for _, s := range safe {
		isPatho, _ := reg.IsPathologicalLiteral(makeLit(s))
		if isPatho {
			t.Errorf("expected %q to be treated as safe selective literal", s)
		}
	}
}
