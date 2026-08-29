package a21_row_lock

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/config"
)

func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("failed to resolve testdata path: %v", err)
	}

	analysistest.Run(t, testdata, Analyzer, "a21")
}

func TestCheckLockingQuery(t *testing.T) {
	keyMap := GetKeyColumns(config.DefaultConfig())

	cases := []struct {
		name      string
		sql       string
		violating bool
	}{
		{
			name:      "SafeSkipLocked",
			sql:       "SELECT id FROM queue WHERE status = 'PENDING' LIMIT 1 FOR UPDATE SKIP LOCKED",
			violating: false,
		},
		{
			name:      "SafeNowait",
			sql:       "SELECT id FROM session WHERE token = $1 FOR UPDATE NOWAIT",
			violating: false,
		},
		{
			name:      "SafePointLookupPK",
			sql:       "SELECT id, balance FROM wallets WHERE id = $1 FOR UPDATE",
			violating: false,
		},
		{
			name:      "SafeNormalSelect",
			sql:       "SELECT id, name FROM users",
			violating: false,
		},
		{
			name:      "UnsafeBlockingQueue",
			sql:       "SELECT id FROM queue WHERE status = 'PENDING' LIMIT 1 FOR UPDATE",
			violating: true,
		},
		{
			name:      "UnsafeNoKeyUpdate",
			sql:       "SELECT id FROM pending_items WHERE tenant_id = $1 LIMIT 5 FOR NO KEY UPDATE",
			violating: true,
		},
	}

	for _, tc := range cases {
		violating, _ := CheckLockingQuery(tc.sql, keyMap)
		if violating != tc.violating {
			t.Errorf("[%s] expected violating=%v, got=%v for SQL:\n%s", tc.name, tc.violating, violating, tc.sql)
		}
	}
}
