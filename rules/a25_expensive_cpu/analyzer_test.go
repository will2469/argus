package a25_expensive_cpu

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

	analysistest.Run(t, testdata, Analyzer, "a25")
}
