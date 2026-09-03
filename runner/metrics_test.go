package runner

import (
	"testing"
)

func TestMetricsTracker_Basic(t *testing.T) {
	tracker := NewMetricsTracker()
	tracker.IncrementScannedFiles(10)
	tracker.IncrementQuerySites(5)
	tracker.IncrementParameterizedSites(15)

	res := tracker.Snapshot()
	if res.ScannedFiles != 10 {
		t.Errorf("expected 10 scanned files, got %d", res.ScannedFiles)
	}
	if res.VerifiedQuerySites != 5 {
		t.Errorf("expected 5 query sites, got %d", res.VerifiedQuerySites)
	}
	if res.VerifiedParameterizedSites != 15 {
		t.Errorf("expected 15 parameterized sites, got %d", res.VerifiedParameterizedSites)
	}
	if res.Score != 100.0 || res.Grade != "A+" {
		t.Errorf("expected clean score 100 A+, got %f %s", res.Score, res.Grade)
	}
}

func TestCountParameters(t *testing.T) {
	sql := "SELECT id, name FROM users WHERE id = $1 AND tenant_id = $2 AND status = $3"
	count := CountParameters(sql)
	if count != 3 {
		t.Errorf("expected 3 placeholders, got %d", count)
	}
}

func TestCalculateCheckedComponents(t *testing.T) {
	// 1. Migration rules with 0 migration files must return 0 (never magic 156)
	migrationRules := []string{"A11", "A13", "A15", "A27", "A28", "A29", "A30"}
	for _, rule := range migrationRules {
		if val := CalculateCheckedComponents(rule, 50, 0, 100); val != 0 {
			t.Errorf("[%s] expected 0 components for 0 migration files, got %d", rule, val)
		}
		if val := CalculateCheckedComponents(rule, 50, 15, 100); val != 15 {
			t.Errorf("[%s] expected 15 components for 15 migration files, got %d", rule, val)
		}
	}

	// 2. Config/lifecycle rules must return totalFiles (never magic 12)
	configRules := []string{"A05", "A06", "A08", "A09", "A10", "A12", "A16", "A22", "A23", "A25"}
	for _, rule := range configRules {
		if val := CalculateCheckedComponents(rule, 50, 0, 100); val != 100 {
			t.Errorf("[%s] expected 100 components, got %d", rule, val)
		}
	}

	// 3. Query rules must return querySites when > 0, else fallback to totalFiles
	if val := CalculateCheckedComponents("A01", 50, 0, 100); val != 50 {
		t.Errorf("[A01] expected 50 query sites, got %d", val)
	}
	if val := CalculateCheckedComponents("A14", 0, 0, 100); val != 100 {
		t.Errorf("[A14] expected fallback to 100 files, got %d", val)
	}
}

