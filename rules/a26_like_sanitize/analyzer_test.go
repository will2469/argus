package a26_like_sanitize

import (
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
