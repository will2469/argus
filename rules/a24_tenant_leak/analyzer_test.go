package a24_tenant_leak

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

	analysistest.Run(t, testdata, Analyzer, "a24")
}

func TestCheckTenantQuery(t *testing.T) {
	tc := &TenantConfig{
		TenantColumn: "tenant_id",
		TenantTables: map[string]bool{
			"users":     true,
			"orders":    true,
			"customers": true,
		},
	}

	cases := []struct {
		sql        string
		violating  bool
		wantReason string
	}{
		{
			sql:       "SELECT id, name FROM users WHERE tenant_id = $1",
			violating: false,
		},
		{
			sql:       "SELECT id, name FROM users WHERE u.tenant_id = $1",
			violating: false,
		},
		{
			sql:       "SELECT id, name FROM users WHERE status = 'ACTIVE'",
			violating: true,
		},
		{
			sql:       "UPDATE orders SET status = 'DONE' WHERE tenant_id = $1 AND id = $2",
			violating: false,
		},
		{
			sql:       "UPDATE orders SET status = 'DONE' WHERE id = $1",
			violating: true,
		},
		{
			sql:       "SELECT id, name FROM lookup_data WHERE active = true",
			violating: false,
		},
		{
			sql:       "SELECT id, name FROM users WHERE id = $1 OR tenant_id = $2",
			violating: true,
		},
		{
			sql:       "SELECT id, name FROM users WHERE (id = $1 OR status = 'A') AND tenant_id = $2",
			violating: false,
		},
		{
			sql:       "SELECT id, name FROM users WHERE (status = 'A' AND tenant_id = $1) OR (status = 'B' AND tenant_id = $1)",
			violating: false,
		},
		{
			sql:       "SELECT id, name FROM users WHERE NOT (tenant_id = $1)",
			violating: true,
		},
		{
			sql:       "SELECT id, name FROM users WHERE id = $1 AND (status = 'A' OR tenant_id = $2)",
			violating: true,
		},
		// P0 Adversarial operator & constraint validation cases
		{
			sql:       "SELECT id FROM users WHERE tenant_id IS NOT NULL",
			violating: true,
		},
		{
			sql:       "SELECT id FROM users WHERE tenant_id > 0",
			violating: true,
		},
		{
			sql:       "SELECT id FROM users WHERE tenant_id != $1",
			violating: true,
		},
		{
			sql:       "SELECT id FROM users WHERE tenant_id <> $1",
			violating: true,
		},
		// P0 Multi-table JOIN isolation cases
		{
			sql:       "SELECT u.id, c.name FROM users u JOIN customers c ON c.id = u.customer_id WHERE u.tenant_id = $1",
			violating: true,
		},
		{
			sql:       "SELECT u.id, c.name FROM users u JOIN customers c ON c.id = u.customer_id WHERE u.tenant_id = $1 AND c.tenant_id = $1",
			violating: false,
		},
		{
			sql:       "SELECT u.id, c.name FROM users u JOIN customers c ON c.id = u.customer_id AND c.tenant_id = $1 WHERE u.tenant_id = $2",
			violating: false,
		},
		{
			sql:       "SELECT u.id, c.name FROM users u JOIN customers c ON c.tenant_id = u.tenant_id WHERE u.tenant_id = $1",
			violating: false,
		},
	}

	for _, c := range cases {
		violating, _ := CheckTenantQuery(c.sql, tc)
		if violating != c.violating {
			t.Errorf("[%s] expected violating=%v, got %v", c.sql, c.violating, violating)
		}
	}
}
