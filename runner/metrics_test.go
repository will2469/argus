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
