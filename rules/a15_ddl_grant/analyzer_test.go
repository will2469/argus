package a15_ddl_grant

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/directives"
)

func TestAnalyzer(t *testing.T) {
	testdata, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatalf("failed to resolve testdata path: %v", err)
	}

	analysistest.Run(t, testdata, Analyzer, "a15")
}

func TestCheckMigration_CompliantDML(t *testing.T) {
	sql := `
GRANT SELECT, INSERT, UPDATE, DELETE ON users TO app_user;
GRANT USAGE, SELECT ON SEQUENCE users_id_seq TO app_user;
`
	reg := NewRoleRegistry([]string{"app_user"})
	issues := CheckMigration("001_safe.sql", sql, nil, reg)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for DML grant, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigration_ForbiddenDDLGrant(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"GrantAll", "GRANT ALL PRIVILEGES ON TABLE users TO app_user;"},
		{"GrantCreate", "GRANT CREATE ON SCHEMA public TO PUBLIC;"},
		{"GrantTruncate", "GRANT TRUNCATE ON users TO app_user;"},
		{"AlterOwner", "ALTER TABLE users OWNER TO app_user;"},
		{"MultilineGrant", "GRANT CREATE, DROP, TRUNCATE\nON SCHEMA public\nTO app_user;"},
	}

	reg := NewRoleRegistry([]string{"app_user", "public"})
	for _, tc := range cases {
		issues := CheckMigration(tc.name+".sql", tc.sql, nil, reg)
		if len(issues) != 1 {
			t.Errorf("expected 1 issue for %s, got %d", tc.name, len(issues))
		}
		if len(issues) > 0 && issues[0].Rule != RuleCode {
			t.Errorf("expected rule %s, got %s", RuleCode, issues[0].Rule)
		}
	}
}

func TestCheckMigration_Ignored(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{
			"CanonicalShortCode",
			"-- argus:ignore-a15 intentional legacy bootstrap schema permissions\nGRANT CREATE ON SCHEMA public TO PUBLIC;",
		},
		{
			"ClauseDotNotation",
			"-- argus:ignore-a15.ddl-grant approved legacy schema grant\nGRANT CREATE ON SCHEMA public TO PUBLIC;",
		},
		{
			"LegacyLongCode",
			"-- argus:ignore ARGUS-A15 intentional legacy bootstrap schema permissions\nGRANT CREATE ON SCHEMA public TO PUBLIC;",
		},
	}

	reg := NewRoleRegistry([]string{"public"})
	for _, tc := range cases {
		dm := directives.ParseSQLDirectives(tc.sql, tc.name+".sql")
		issues := CheckMigration(tc.name+".sql", tc.sql, dm, reg)
		if len(issues) != 0 {
			t.Fatalf("[%s] expected 0 issues when ignored, got %d: %v", tc.name, len(issues), issues)
		}
	}
}
