package runner

import (
	"path/filepath"
	"testing"

	"github.com/will2469/argus/rules"
	"github.com/will2469/argus/shared/config"
)

func TestGoldenCorpus_AuditBehavior(t *testing.T) {
	testdata, err := filepath.Abs("../testdata/src/golden")
	if err != nil {
		t.Fatalf("failed to resolve testdata golden path: %v", err)
	}

	cfg := config.DefaultConfig()

	auditCfg := AuditConfig{
		RootDir:       testdata,
		ScanDirs:      []string{testdata},
		MigrationDirs: []string{"/tmp/empty_mig"},
		Config:        cfg,
		Analyzers:     rules.AllAnalyzers,
	}

	result, err := RunAuditWithConfig(auditCfg)
	if err != nil {
		t.Fatalf("RunAuditWithConfig failed: %v", err)
	}

	// Expected exact 4 issues:
	// 1. A17: nPlusOne (direct loop)
	// 2. A17: nPlusOneDeep (transitive helper query loop)
	// 3. A26: unsafeSearch (unsanitized wildcard in LIKE)
	// 4. A24: unsafeTenant (tenant_id IS NOT NULL is non-isolating)
	if len(result.Issues) != 4 {
		t.Fatalf("expected exactly 4 issues in golden corpus, got %d: %+v", len(result.Issues), result.Issues)
	}

	ruleCounts := make(map[string]int)
	for _, issue := range result.Issues {
		ruleCounts[issue.Rule]++
		t.Logf("Golden Issue: Line %d [%s] %s", issue.Line, issue.Rule, issue.Message)
	}

	if ruleCounts["FORBIDDEN_QUERY_IN_LOOP"] != 2 {
		t.Errorf("expected 2 A17 FORBIDDEN_QUERY_IN_LOOP issues, got %d", ruleCounts["FORBIDDEN_QUERY_IN_LOOP"])
	}
	if ruleCounts["LIKE_WILDCARD_INJECTION"] != 1 {
		t.Errorf("expected 1 A26 LIKE_WILDCARD_INJECTION issue, got %d", ruleCounts["LIKE_WILDCARD_INJECTION"])
	}
	if ruleCounts["TENANT_ISOLATION_LEAK"] != 1 {
		t.Errorf("expected 1 A24 TENANT_ISOLATION_LEAK issue, got %d", ruleCounts["TENANT_ISOLATION_LEAK"])
	}
}
