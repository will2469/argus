package a19_unbounded_limit

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/config"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve rootDir: %v", err)
	}

	analysistest.Run(t, rootDir, Analyzer,
		"./tests/correctness/a19/positive",
		"./tests/correctness/a19/negative",
	)
}

func TestCheckUnboundedQuery(t *testing.T) {
	tableMap := GetHighCardinalityTables(config.DefaultConfig())
	keyMap := GetKeyColumns(config.DefaultConfig())

	cases := []struct {
		name        string
		sql         string
		unbounded   bool
		targetTable string
	}{
		{
			name:        "UnboundedSelect",
			sql:         "SELECT id, action FROM audit_logs WHERE tenant_id = $1",
			unbounded:   true,
			targetTable: "audit_logs",
		},
		{
			name:        "BoundedWithLimit",
			sql:         "SELECT id, action FROM audit_logs WHERE tenant_id = $1 LIMIT 100",
			unbounded:   false,
			targetTable: "",
		},
		{
			name:        "BoundedWithFetchFirst",
			sql:         "SELECT id, action FROM audit_logs FETCH FIRST 50 ROWS ONLY",
			unbounded:   false,
			targetTable: "",
		},
		{
			name:        "ScalarAggregate",
			sql:         "SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1",
			unbounded:   false,
			targetTable: "",
		},
		{
			name:        "AggregateWithGroupBy",
			sql:         "SELECT action, COUNT(*) FROM audit_logs GROUP BY action",
			unbounded:   true,
			targetTable: "audit_logs",
		},
		{
			name:        "PointLookupPrimaryKey",
			sql:         "SELECT id, action FROM audit_logs WHERE id = $1",
			unbounded:   false,
			targetTable: "",
		},
		{
			name:        "NonHighCardinalityTable",
			sql:         "SELECT id, name FROM ref_status",
			unbounded:   false,
			targetTable: "",
		},
	}

	for _, tc := range cases {
		gotUnbounded, gotTable := CheckUnboundedQuery(tc.sql, tableMap, keyMap)
		if gotUnbounded != tc.unbounded {
			t.Errorf("[%s] expected unbounded=%v, got=%v for SQL:\n%s", tc.name, tc.unbounded, gotUnbounded, tc.sql)
		}
		if gotTable != tc.targetTable {
			t.Errorf("[%s] expected targetTable=%q, got=%q", tc.name, tc.targetTable, gotTable)
		}
	}
}
