// Package a27_concurrent_index provides a standalone runner to audit migration directories directly.
package a27_concurrent_index

import (
	"github.com/will2469/argus/shared/migration"
)

// ScanMigrationDir scans a directory of SQL migration files for non-concurrent index operations on existing tables.
func ScanMigrationDir(dir string) ([]migration.Issue, error) {
	return migration.ScanDirectory(dir, CheckMigration)
}
