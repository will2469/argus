// Package a28_constraint_lock provides a standalone runner to audit migration directories directly.
package a28_constraint_lock

import (
	"github.com/will2469/argus/shared/migration"
)

// ScanMigrationDir scans a directory of SQL migration files for direct table-locking constraint additions.
func ScanMigrationDir(dir string) ([]migration.Issue, error) {
	return migration.ScanDirectory(dir, CheckMigration)
}
