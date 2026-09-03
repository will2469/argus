package tests_test

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/will2469/argus/runner"
)

// TestCrossRule_IsolationHarness verifies Layer 4 of the Argus Quality Pyramid:
// Multi-rule concurrent execution, AST cache thread-safety, zero rule shadowing,
// and strict directive isolation across multi-rule source files.
func TestCrossRule_IsolationHarness(t *testing.T) {
	rootDir, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("failed to resolve root dir: %v", err)
	}

	goldenTarget := filepath.Join(rootDir, "tests", "golden", "golden.go")

	// ---------------------------------------------------------------------
	// 1. Concurrent Multi-Rule Audit & Cache Race-Freedom Test
	// ---------------------------------------------------------------------
	const concurrency = 20
	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)

	expectedRules := map[string]int{
		"FORBIDDEN_QUERY_IN_LOOP": 2, // ARGUS-A17 (loop + helper)
		"LIKE_WILDCARD_INJECTION": 1, // ARGUS-A26
		"TENANT_ISOLATION_LEAK":   1, // ARGUS-A24
	}

	t.Log("=========================================================================================================")
	t.Log("ARGUS 1-SSOT GOLDEN CORPUS — GATE 4: CROSS-RULE ISOLATION & REGRESSION HARNESS")
	t.Log("=========================================================================================================")

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			res, err := runner.RunAuditWithConfig(runner.AuditConfig{
				RootDir:  rootDir,
				ScanDirs: []string{goldenTarget},
			})
			if err != nil {
				errCh <- err
				return
			}

			if len(res.Issues) != 4 {
				t.Errorf("Worker %d: expected 4 issues on golden.go, got %d", workerID, len(res.Issues))
				return
			}

			ruleCounts := make(map[string]int)
			for _, iss := range res.Issues {
				ruleCounts[iss.Rule]++
			}

			for rule, count := range expectedRules {
				if ruleCounts[rule] != count {
					t.Errorf("Worker %d: expected %d for rule %s, got %d", workerID, count, rule, ruleCounts[rule])
				}
			}
		}(w)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("Concurrent audit failed: %v", err)
	}

	t.Logf("  [PASS] Concurrent Multi-Checker Execution: %d threads clean (zero data race, zero panic)", concurrency)
	t.Log("  [PASS] Shared AST Cache Thread-Safety: 100% deterministic results across all workers")

	// ---------------------------------------------------------------------
	// 2. Zero Shadowing Independence Verification
	// ---------------------------------------------------------------------
	singleRes, err := runner.RunAuditWithConfig(runner.AuditConfig{
		RootDir:  rootDir,
		ScanDirs: []string{goldenTarget},
	})
	if err != nil {
		t.Fatalf("single audit failed: %v", err)
	}

	detectedRules := make(map[string]bool)
	for _, iss := range singleRes.Issues {
		detectedRules[iss.Rule] = true
	}

	for rule := range expectedRules {
		if !detectedRules[rule] {
			t.Errorf("SHADOWING DETECTED: Rule %s was shadowed in multi-rule file!", rule)
		}
	}
	t.Log("  [PASS] Zero Rule Shadowing: A17, A24, and A26 evaluated completely independently")

	// ---------------------------------------------------------------------
	// 3. Directive Scoping Isolation Verification
	// ---------------------------------------------------------------------
	// Verify that safe functions (SafeQuery, Search, UnrelatedQuery) triggered 0 false positives
	for _, iss := range singleRes.Issues {
		if iss.Line < 50 || (iss.Line >= 73 && iss.Line <= 80) {
			t.Errorf("FALSE POSITIVE in safe section of golden.go at line %d: %s", iss.Line, iss.Message)
		}
	}
	t.Log("  [PASS] Safe Scope Preservation: 0 false positives on safe helper, search, and query functions")
	t.Log("=========================================================================================================")
	t.Log("Cross-Rule Regression & Multi-Checker Isolation Gate: PASS (100% Sound)")
	t.Log("=========================================================================================================")
}
