package a13_missing_down_migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanDirectory_SchemaQualification_Mismatch(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte("CREATE TABLE public.users (id int);"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.down.sql"), []byte("DROP TABLE audit.users;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for schema qualification mismatch, got %d: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "missing DROP TABLE for table \"users\"") {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestScanDirectory_SchemaQualification_Match(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_audit.up.sql"), []byte("CREATE TABLE audit.users (id int);"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_audit.down.sql"), []byte("DROP TABLE audit.users;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for matching schema qualification, got %d: %v", len(issues), issues)
	}
}

func TestScanDirectory_Unqualified_MatchesPublic(t *testing.T) {
	tempDir := t.TempDir()

	// Unqualified UP vs explicit public DOWN
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.up.sql"), []byte("CREATE TABLE users (id int);"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_users.down.sql"), []byte("DROP TABLE public.users;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for unqualified vs public match, got %d: %v", len(issues), issues)
	}
}

func TestScanDirectory_AlterColumnType_ProvenInverse(t *testing.T) {
	tempDir := t.TempDir()

	// 001 creates users with age integer
	_ = os.WriteFile(filepath.Join(tempDir, "001_init.up.sql"), []byte("CREATE TABLE users (id int, age integer);"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_init.down.sql"), []byte("DROP TABLE users;"), 0644)

	// 002 alters age to bigint, down restores to integer
	_ = os.WriteFile(filepath.Join(tempDir, "002_alter.up.sql"), []byte("ALTER TABLE users ALTER COLUMN age TYPE bigint;"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "002_alter.down.sql"), []byte("ALTER TABLE users ALTER COLUMN age TYPE integer;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for proven type inverse, got %d: %v", len(issues), issues)
	}
}

func TestScanDirectory_AlterColumnType_WrongInverse(t *testing.T) {
	tempDir := t.TempDir()

	// 001 creates users with age integer
	_ = os.WriteFile(filepath.Join(tempDir, "001_init.up.sql"), []byte("CREATE TABLE users (id int, age integer);"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_init.down.sql"), []byte("DROP TABLE users;"), 0644)

	// 002 alters age to bigint, but down alters to text (not integer!)
	_ = os.WriteFile(filepath.Join(tempDir, "002_alter.up.sql"), []byte("ALTER TABLE users ALTER COLUMN age TYPE bigint;"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "002_alter.down.sql"), []byte("ALTER TABLE users ALTER COLUMN age TYPE text;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for wrong type inverse, got %d: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "does not restore original type \"integer\"") {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestScanDirectory_AlterColumnType_IdenticalType(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_alter.up.sql"), []byte("ALTER TABLE users ALTER COLUMN age TYPE bigint;"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_alter.down.sql"), []byte("ALTER TABLE users ALTER COLUMN age TYPE bigint;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for identical type alteration, got %d: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "altered to the same type \"bigint\"") {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestScanDirectory_AlterColumnType_UnknownOriginalType(t *testing.T) {
	tempDir := t.TempDir()

	// Unknown original type: table was never declared in migration history
	_ = os.WriteFile(filepath.Join(tempDir, "001_alter.up.sql"), []byte("ALTER TABLE legacy_users ALTER COLUMN age TYPE bigint;"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_alter.down.sql"), []byte("ALTER TABLE legacy_users ALTER COLUMN age TYPE text;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for unknown original type, got %d: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "cannot prove reversibility of type alteration") {
		t.Errorf("unexpected message: %s", issues[0].Message)
	}
}

func TestScanDirectory_AlterColumnType_IgnoredDirective(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "001_alter.up.sql"), []byte("ALTER TABLE legacy_users ALTER COLUMN age TYPE bigint;"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "001_alter.down.sql"), []byte("-- argus:ignore-a13 ADR-0099 lossy schema migration\nALTER TABLE legacy_users ALTER COLUMN age TYPE text;"), 0644)

	issues := ScanDirectory(tempDir, nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues when ignored with directive, got %d: %v", len(issues), issues)
	}
}
