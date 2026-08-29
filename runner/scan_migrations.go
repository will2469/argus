package runner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/will2469/argus/rules/a11_destructive_migration"
	"github.com/will2469/argus/rules/a13_missing_down_migration"
	"github.com/will2469/argus/rules/a15_ddl_grant"
	"github.com/will2469/argus/rules/a27_concurrent_index"
	"github.com/will2469/argus/rules/a28_constraint_lock"
	"github.com/will2469/argus/rules/a29_unindexed_fk"
	"github.com/will2469/argus/rules/a30_timestamptz"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// scanMigrationDirectories runs all migration checker rules across specified migration directories.
func scanMigrationDirectories(migrationDirs []string, rootDir string, tracker *MetricsTracker, cfg *config.Config) {
	reg := a15_ddl_grant.FromConfig(cfg)

	for _, dir := range migrationDirs {
		targetDir := dir
		if !filepath.IsAbs(targetDir) {
			targetDir = filepath.Join(rootDir, targetDir)
		}

		entries, err := os.ReadDir(targetDir)
		if err != nil {
			continue
		}

		var sqlFiles []string
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
				continue
			}
			sqlFiles = append(sqlFiles, filepath.Join(targetDir, entry.Name()))
		}
		tracker.IncrementScannedFiles(len(sqlFiles))

		dm := directives.NewDirectiveMap()

		// 1. Check A13: Missing Down Migrations
		for _, issue := range a13_missing_down_migration.ScanDirectory(targetDir, dm) {
			addMigrationIssue(issue, "ARGUS-A13", rootDir, tracker)
		}

		// 2. Check A29: Unindexed Foreign Keys across migration directory
		for _, issue := range a29_unindexed_fk.ScanMigrationDir(targetDir, dm, cfg) {
			addMigrationIssue(issue, "ARGUS-A29", rootDir, tracker)
		}

		// 3. Per-file migration checks: A11, A15, A27, A28, A30
		for _, file := range sqlFiles {
			if !strings.HasSuffix(file, ".up.sql") {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}
			content := string(data)
			fileDm := directives.ParseSQLDirectives(content, file)

			// A11: Destructive Migrations
			for _, issue := range a11_destructive_migration.CheckMigration(file, content, fileDm) {
				addMigrationIssue(issue, "ARGUS-A11", rootDir, tracker)
			}

			// A15: Forbidden DDL App Role Grants
			for _, issue := range a15_ddl_grant.CheckMigration(file, content, fileDm, reg) {
				addMigrationIssue(issue, "ARGUS-A15", rootDir, tracker)
			}

			// A27: Non-concurrent Index Creation
			for _, issue := range a27_concurrent_index.CheckMigration(file, content, fileDm) {
				addMigrationIssue(issue, "ARGUS-A27", rootDir, tracker)
			}

			// A28: Table Locking Constraint Addition
			for _, issue := range a28_constraint_lock.CheckMigration(file, content, fileDm) {
				addMigrationIssue(issue, "ARGUS-A28", rootDir, tracker)
			}

			// A30: Timestamp without Timezone
			for _, issue := range a30_timestamptz.CheckMigration(file, content, fileDm) {
				addMigrationIssue(issue, "ARGUS-A30", rootDir, tracker)
			}
		}
	}
}

func addMigrationIssue(issue migration.Issue, ruleCode string, rootDir string, tracker *MetricsTracker) {
	relPath, err := filepath.Rel(rootDir, issue.Filename)
	if err != nil {
		relPath = issue.Filename
	}
	tracker.AddIssue(Issue{
		File:     relPath,
		Line:     issue.Line,
		Rule:     ruleCode,
		Message:  issue.Message,
		Category: "migration",
	})
}
