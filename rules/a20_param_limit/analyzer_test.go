package a20_param_limit

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

	analysistest.Run(t, testdata, Analyzer, "a20")
}

func TestGetRemediationHelp(t *testing.T) {
	cases := []struct {
		kind     DynamicBatchKind
		contains string
	}{
		{BatchKindDynamicValues, "pgx.CopyFrom"},
		{BatchKindDynamicInClause, "ANY($1)"},
		{BatchKindNone, "65,535"},
	}

	for _, tc := range cases {
		got := GetRemediationHelp(tc.kind)
		if got == "" {
			t.Errorf("[%v] expected non-empty remediation message", tc.kind)
		}
	}
}
