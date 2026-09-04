package a15_ddl_grant

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/will2469/argus/shared/directives"
)

func TestAnalyzer(t *testing.T) {
	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("failed to resolve rootDir: %v", err)
	}

	analysistest.Run(t, rootDir, Analyzer,
		"./tests/migration/a15/positive",
		"./tests/migration/a15/negative",
	)
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

func TestCheckMigration_SequenceAndFunctionAllNotDDL(t *testing.T) {
	// ALL on SEQUENCE only conveys USAGE, SELECT, UPDATE (zero DDL).
	// ALL on FUNCTION only conveys EXECUTE (zero DDL).
	sql := `
GRANT ALL ON SEQUENCE users_id_seq TO app_user;
GRANT ALL ON FUNCTION calculate_total(int) TO app_user;
`
	reg := NewRoleRegistry([]string{"app_user"})
	issues := CheckMigration("seq_func_safe.sql", sql, nil, reg)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for ALL on sequence/function, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigration_ForbiddenDDLGrant(t *testing.T) {
	cases := []struct {
		name        string
		sql         string
		msgContains string
	}{
		{"GrantAll", "GRANT ALL PRIVILEGES ON TABLE users TO app_user;", "runtime role \"app_user\""},
		{"GrantCreatePublic", "GRANT CREATE ON SCHEMA public TO PUBLIC;", "PUBLIC pseudo-role"},
		{"GrantTruncate", "GRANT TRUNCATE ON users TO app_user;", "runtime role \"app_user\""},
		{"AlterOwner", "ALTER TABLE users OWNER TO app_user;", "runtime app role \"app_user\""},
		{"AlterOwnerQuoted", "ALTER TABLE users OWNER TO \"app_user\";", "runtime app role \"app_user\""},
		{"AlterOwnerPublic", "ALTER TABLE users OWNER TO PUBLIC;", "PUBLIC pseudo-role"},
		{"GrantAdminRole", "GRANT superuser TO app_user;", "Forbidden administrative role grant \"superuser\""},
		{"MultilineGrant", "GRANT CREATE, DROP, TRUNCATE\nON SCHEMA public\nTO app_user;", "runtime role \"app_user\""},
	}

	reg := NewRoleRegistry([]string{"app_user", "public"})
	for _, tc := range cases {
		issues := CheckMigration(tc.name+".sql", tc.sql, nil, reg)
		if len(issues) != 1 {
			t.Errorf("expected 1 issue for %s, got %d", tc.name, len(issues))
			continue
		}
		if issues[0].Rule != RuleCode {
			t.Errorf("expected rule %s, got %s", RuleCode, issues[0].Rule)
		}
		if !strings.Contains(issues[0].Message, tc.msgContains) {
			t.Errorf("[%s] expected message to contain %q, got: %s", tc.name, tc.msgContains, issues[0].Message)
		}
	}
}

func TestCheckMigration_CompliantOwner(t *testing.T) {
	sql := `
ALTER TABLE users OWNER TO CURRENT_USER;
ALTER TABLE users OWNER TO migrator_admin;
`
	reg := NewRoleRegistry([]string{"app_user"})
	issues := CheckMigration("owner_safe.sql", sql, nil, reg)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for admin/CURRENT_USER owner, got %d: %v", len(issues), issues)
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
