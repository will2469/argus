package a14_select_star

import (
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
		"./tests/correctness/a14/positive",
		"./tests/correctness/a14/negative",
	)
}

func TestHasForbiddenSelectStar(t *testing.T) {
	cases := []struct {
		name      string
		sql       string
		forbidden bool
	}{
		{"PlainStar", "SELECT * FROM users", true},
		{"AliasStar", "SELECT u.* FROM users u", true},
		{"CTEStar", "WITH cte AS (SELECT * FROM a) SELECT id FROM cte", true},
		{"ExplicitCols", "SELECT id, name FROM users", false},
		{"CountStar", "SELECT COUNT(*) FROM users", false},
		{"ExistsStar", "SELECT id FROM users WHERE EXISTS (SELECT * FROM orders WHERE orders.user_id = users.id)", false},
		{"NotExistsStar", "SELECT id FROM users WHERE NOT EXISTS (SELECT * FROM orders WHERE orders.user_id = users.id)", false},
	}

	for _, tc := range cases {
		got := HasForbiddenSelectStar(tc.sql)
		if got != tc.forbidden {
			t.Errorf("[%s] expected forbidden=%v, got=%v for SQL:\n%s", tc.name, tc.forbidden, got, tc.sql)
		}
	}
}
