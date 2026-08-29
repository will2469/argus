// Package a11_destructive_migration provides a standalone runner to audit migration directories directly.
package a11_destructive_migration

import (
	"github.com/will2469/argus/shared/migration"
)

// ScanMigrationDir scans a directory of SQL migration files for destructive operations.
func ScanMigrationDir(dir string) ([]migration.Issue, error) {
	return migration.ScanDirectory(dir, CheckMigration)
}
