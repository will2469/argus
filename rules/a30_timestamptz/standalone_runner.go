// Package a30_timestamptz provides a standalone runner to audit migration directories directly.
package a30_timestamptz

import (
	"github.com/will2469/argus/shared/migration"
)

// ScanMigrationDir scans a directory of SQL migration files for bare TIMESTAMP column definitions.
func ScanMigrationDir(dir string) ([]migration.Issue, error) {
	return migration.ScanDirectory(dir, CheckMigration)
}
