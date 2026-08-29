package a29_unindexed_fk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
)

func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("failed to resolve testdata path: %v", err)
	}

	analysistest.Run(t, testdata, Analyzer, "a29")
}

func TestCheckMigrations_IndexedFKCompliant(t *testing.T) {
	files := map[string]string{
		"001_users.sql": `
CREATE TABLE users (id UUID PRIMARY KEY);
`,
		"002_orders.sql": `
CREATE TABLE orders (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id)
);

CREATE INDEX idx_orders_user_id ON orders (user_id);
`,
	}

	issues := CheckMigrations(files, nil, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for indexed FK, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigrations_UnindexedFKViolation(t *testing.T) {
	files := map[string]string{
		"001_users.sql": `
CREATE TABLE users (id UUID PRIMARY KEY);
`,
		"002_orders.sql": `
CREATE TABLE orders (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id)
);
`,
	}

	issues := CheckMigrations(files, nil, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for unindexed FK, got %d", len(issues))
	}
	if issues[0].Rule != RuleCode {
		t.Errorf("expected rule %s, got %s", RuleCode, issues[0].Rule)
	}
}

func TestCheckMigrations_LeadingPKCompliant(t *testing.T) {
	files := map[string]string{
		"001_user_roles.sql": `
CREATE TABLE user_roles (
    user_id UUID NOT NULL,
    role_id UUID NOT NULL,
    PRIMARY KEY (user_id, role_id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);
`,
	}

	issues := CheckMigrations(files, nil, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when FK is leading column of PK, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigrations_IgnoredParentPrefix(t *testing.T) {
	files := map[string]string{
		"001_ref.sql": `
CREATE TABLE ref_status (code INT PRIMARY KEY);
CREATE TABLE items (
    id UUID PRIMARY KEY,
    status_code INT NOT NULL REFERENCES ref_status(code)
);
`,
	}

	cfg := config.DefaultConfig()
	issues := CheckMigrations(files, nil, cfg)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for ref_ parent prefix, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigrations_Ignored(t *testing.T) {
	files := map[string]string{
		"001_legacy.sql": `
CREATE TABLE orders (
    id UUID PRIMARY KEY,
    -- argus:ignore ARGUS-A29 legacy read-only audit lookup
    user_id UUID NOT NULL REFERENCES users(id)
);
`,
	}

	dm := directives.ParseSQLDirectives(files["001_legacy.sql"], "001_legacy.sql")
	issues := CheckMigrations(files, dm, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when ignored, got %d: %v", len(issues), issues)
	}
}

func TestScanMigrationDir_TestData(t *testing.T) {
	testDir := "../../testdata/src/a29/migrations"
	dm := directives.NewDirectiveMap()
	cfg := config.DefaultConfig()

	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			data, _ := os.ReadFile(filepath.Join(testDir, entry.Name()))
			fileDm := directives.ParseSQLDirectives(string(data), entry.Name())
			for l := 1; l <= strings.Count(string(data), "\n")+1; l++ {
				if fileDm.IsLineIgnored(entry.Name(), l, RuleCode) {
					dm.AddDirective(entry.Name(), l, RuleCode, "test ignore")
				}
			}
		}
	}

	issues := ScanMigrationDir(testDir, dm, cfg)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue from testdata, got %d: %v", len(issues), issues)
	}
}
