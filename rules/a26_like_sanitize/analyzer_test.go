package a26_like_sanitize

import (
	"go/ast"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("failed to resolve testdata path: %v", err)
	}

	analysistest.Run(t, testdata, Analyzer, "a26")
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
	// Call expression AST helpers
	makeCall := func(name string) *ast.CallExpr {
		return &ast.CallExpr{
			Fun: &ast.Ident{Name: name},
		}
	}

	if isSanitizerCall(makeCall("ReplaceAll")) {
		t.Errorf("ReplaceAll must NOT be considered a safe LIKE sanitizer")
	}
	if isSanitizerCall(makeCall("strings.ReplaceAll")) {
		t.Errorf("strings.ReplaceAll must NOT be considered a safe LIKE sanitizer")
	}

	validSanitizers := []string{
		"SanitizeLike", "SanitizeLikePattern", "SanitizeLikeWildcards",
		"FormatLikeContains", "FormatLikePrefix", "FormatLikeSuffix",
		"EscapeLike", "EscapeLikePattern", "QuoteLike", "CleanLikePattern",
	}
	for _, name := range validSanitizers {
		if !isSanitizerCall(makeCall(name)) {
			t.Errorf("expected %s to be recognized as a valid sanitizer", name)
		}
	}
}

