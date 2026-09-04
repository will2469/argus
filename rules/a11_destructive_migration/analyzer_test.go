package a11_destructive_migration

import (
	"path/filepath"
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
		"./tests/migration/a11/positive",
		"./tests/migration/a11/negative",
	)
}

func TestCheckMigration_Compliant(t *testing.T) {
	sql := `
CREATE TABLE users (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);

ALTER TABLE users ADD COLUMN phone VARCHAR(20) DEFAULT '';
`
	issues := CheckMigration("001_safe.sql", sql, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for safe migration, got %d: %v", len(issues), issues)
	}
}

func TestCheckMigration_DestructiveViolations(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"DropTable", "DROP TABLE users;"},
		{"DropColumn", "ALTER TABLE users DROP COLUMN phone;"},
		{"AlterType", "ALTER TABLE users ALTER COLUMN age TYPE BIGINT;"},
		{"Truncate", "TRUNCATE TABLE users;"},
		{"AddNotNullNoDefault", "ALTER TABLE users ADD COLUMN email VARCHAR(100) NOT NULL;"},
		{"DropSchema", "DROP SCHEMA public CASCADE;"},
		{"DropDatabase", "DROP DATABASE testdb;"},
		{"DropSequence", "DROP SEQUENCE order_seq;"},
		{"DropView", "DROP VIEW active_users;"},
		{"DropType", "DROP TYPE user_role;"},
		{"DropConstraint", "ALTER TABLE orders DROP CONSTRAINT fk_user;"},
		{"SetNotNull", "ALTER TABLE users ALTER COLUMN email SET NOT NULL;"},
		{"DetachPartition", "ALTER TABLE orders DETACH PARTITION orders_2024;"},
	}

	for _, tc := range cases {
		issues := CheckMigration(tc.name+".sql", tc.sql, nil)
		if len(issues) != 1 {
			t.Errorf("expected 1 issue for %s, got %d", tc.name, len(issues))
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
			"-- argus:ignore-a11 approved contract phase cleanup of deprecated column\nALTER TABLE users DROP COLUMN legacy_bio;",
		},
		{
			"ClauseDotNotation",
			"-- argus:ignore-a11.drop-column approved contract phase cleanup of deprecated column\nALTER TABLE users DROP COLUMN legacy_bio;",
		},
		{
			"LegacyLongCode",
			"-- argus:ignore ARGUS-A11 approved contract phase cleanup of deprecated column\nALTER TABLE users DROP COLUMN legacy_bio;",
		},
	}

	for _, tc := range cases {
		dm := directives.ParseSQLDirectives(tc.sql, tc.name+".sql")
		issues := CheckMigration(tc.name+".sql", tc.sql, dm)
		if len(issues) != 0 {
			t.Fatalf("[%s] expected 0 issues when ignored, got %d: %v", tc.name, len(issues), issues)
		}
	}
}

func TestContractTag_Valid(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{
			"QualifiedReleaseTask",
			"-- argus:contract release_v2_cleanup\nALTER TABLE users DROP COLUMN old_token;",
		},
		{
			"StructuredKeyValues",
			"-- argus:contract phase=contract release=v2.0.0 issue=DB-101 approved_by=dba\nALTER TABLE users DROP COLUMN old_token;",
		},
		{
			"FileHeaderPhase",
			"-- argus:phase contract release=v2.0.0 issue=MIG-123\nALTER TABLE users DROP COLUMN old_token;",
		},
	}

	for _, tc := range cases {
		issues := CheckMigration(tc.name+".sql", tc.sql, nil)
		if len(issues) != 0 {
			t.Fatalf("[%s] expected 0 issues for valid contract evidence, got %d", tc.name, len(issues))
		}
	}
}

func TestContractTag_DummyBypassRejected(t *testing.T) {
	dummyBypasses := []struct {
		name string
		sql  string
	}{
		{"AnythingDummy", "-- argus:contract anything\nDROP TABLE users;"},
		{"TestDummy", "-- argus:contract test\nDROP TABLE users;"},
		{"FooDummy", "-- argus:contract foo\nDROP TABLE users;"},
		{"NoAccountability", "-- argus:contract v1.0.0\nDROP TABLE users;"},
	}

	for _, tc := range dummyBypasses {
		issues := CheckMigration(tc.name+".sql", tc.sql, nil)
		if len(issues) == 0 {
			t.Errorf("[%s] CORRECTION NEEDED: dummy contract bypass must be REJECTED, but was allowed", tc.name)
		}
	}
}
