package a09_advisory_lock

import (
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
		"./tests/correctness/a09/positive",
		"./tests/correctness/a09/negative",
	)
}

func TestInspectAdvisorySQL_Unit(t *testing.T) {
	tests := []struct {
		sql           string
		expectSession bool
		expectIntKey  bool
	}{
		{"SELECT pg_advisory_xact_lock($1)", false, false},
		{"SELECT pg_advisory_xact_lock($1, $2)", false, false},
		{"SELECT pg_advisory_xact_lock(1001, $1)", false, false},
		{"SELECT pg_advisory_xact_lock(namespace_id, resource_id)", false, false},
		{"SELECT pg_advisory_lock($1)", true, false},
		{"SELECT pg_try_advisory_lock($1)", true, false},
		{"SELECT pg_advisory_xact_lock(1)", false, true},
		{"SELECT pg_advisory_xact_lock(1001, 2)", false, true},
		{"SELECT pg_advisory_xact_lock($1, 42)", false, true},
	}

	for _, tc := range tests {
		violations := InspectAdvisorySQL(tc.sql)
		hasSession := false
		hasIntKey := false
		for _, v := range violations {
			if v.Type == ViolationSessionLock {
				hasSession = true
			}
			if v.Type == ViolationHardcodedIntKey {
				hasIntKey = true
			}
		}
		if hasSession != tc.expectSession {
			t.Errorf("for query %q, expected session lock=%v, got=%v", tc.sql, tc.expectSession, hasSession)
		}
		if hasIntKey != tc.expectIntKey {
			t.Errorf("for query %q, expected hardcoded key=%v, got=%v", tc.sql, tc.expectIntKey, hasIntKey)
		}
	}
}
