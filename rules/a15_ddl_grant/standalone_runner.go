package a15_ddl_grant

import (
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// ScanDirectory checks all .up.sql migrations in a directory for forbidden DDL grants or ownership changes.
func ScanDirectory(migrationDir string, reg *RoleRegistry) []migration.Issue {
	issues, _ := migration.ScanDirectory(migrationDir, func(filename, content string, dm *directives.DirectiveMap) []migration.Issue {
		return CheckMigration(filename, content, dm, reg)
	})
	return issues
}
