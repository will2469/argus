package runner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/will2469/argus/rules/a05_audit_immutability"
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
	"github.com/will2469/argus/shared/sqlparser"
)

// scanMigrationDirectories runs all migration checker rules across specified migration directories.
func scanMigrationDirectories(migrationDirs []string, rootDir string, tracker *MetricsTracker, cfg *config.Config, fsys fs.FS) {
	reg := a15_ddl_grant.FromConfig(cfg)

	for _, dir := range migrationDirs {
		var targetDir string
		var sqlFiles []string
		if fsys != nil {
			targetDir = filepath.ToSlash(filepath.Clean(dir))
			if targetDir == "" {
				targetDir = "."
			}
			sqlFiles = findFilesWithExtFS(fsys, targetDir, ".sql")
		} else {
			targetDir = dir
			if !filepath.IsAbs(targetDir) {
				targetDir = filepath.Join(rootDir, targetDir)
			}
			sqlFiles = findFilesWithExt(targetDir, ".sql")
		}

		tracker.IncrementMigrationFiles(len(sqlFiles))

		dm := directives.NewDirectiveMap()

		// 1. Check A13: Missing Down Migrations
		if isRuleActive(cfg, a13_missing_down_migration.RuleCode) {
			var a13Issues []migration.Issue
			if fsys != nil {
				a13Issues = a13_missing_down_migration.ScanDirectoryFS(fsys, targetDir, dm)
			} else {
				a13Issues = a13_missing_down_migration.ScanDirectory(targetDir, dm)
			}
			for _, issue := range a13Issues {
				addMigrationIssue(issue, "ARGUS-A13", rootDir, tracker)
			}
		}

		// 2. Check A29: Unindexed Foreign Keys across migration directory
		if isRuleActive(cfg, a29_unindexed_fk.RuleCode) {
			var a29Issues []migration.Issue
			if fsys != nil {
				a29Issues = a29_unindexed_fk.ScanMigrationDirFS(fsys, targetDir, dm, cfg)
			} else {
				a29Issues = a29_unindexed_fk.ScanMigrationDir(targetDir, dm, cfg)
			}
			for _, issue := range a29Issues {
				addMigrationIssue(issue, "ARGUS-A29", rootDir, tracker)
			}
		}

		// 3. Per-file migration checks: A05, A11, A15, A27, A28, A30
		checkDownA05 := cfg.GetBool(a05_audit_immutability.RuleCode, "check_down_migrations", false)
		for _, file := range sqlFiles {
			isUp := strings.HasSuffix(file, ".up.sql")
			isDown := strings.HasSuffix(file, ".down.sql")
			if !isUp && (!checkDownA05 || !isDown) {
				continue
			}
			var data []byte
			var err error
			if fsys != nil {
				data, err = fs.ReadFile(fsys, filepath.ToSlash(file))
			} else {
				data, err = os.ReadFile(file)
			}
			if err != nil {
				continue
			}
			content := string(data)

			// Pre-validate PostgreSQL AST parseability (prevent silent failure / false negatives)
			if _, err := sqlparser.Parse(content); err != nil {
				msg := fmt.Sprintf("unable to analyze migration; PostgreSQL parser rejected statement: %v", err)
				if cfg == nil || cfg.IsStrictMode() {
					addMigrationIssue(migration.Issue{
						Filename: file,
						Line:     1,
						Message:  msg,
					}, "ARGUS-E001", rootDir, tracker)
				} else {
					fmt.Fprintf(os.Stderr, "⚠️ [ARGUS-E001] WARNING: %s: %s (permissive mode)\n", file, msg)
				}
				continue
			}

			fileDm := directives.ParseSQLDirectives(content, file)

			// A05: Audit Table Immutability
			if isRuleActive(cfg, a05_audit_immutability.RuleCode) && (isUp || (isDown && checkDownA05)) {
				tables := cfg.GetStringSlice(a05_audit_immutability.RuleCode, "audit_tables", []string{"audit_logs", "security_events"})
				auditMap := make(map[string]bool)
				for _, t := range tables {
					auditMap[strings.ToLower(strings.TrimSpace(t))] = true
				}
				for _, issue := range a05_audit_immutability.CheckMigration(file, content, fileDm, auditMap) {
					addMigrationIssue(issue, "ARGUS-A05", rootDir, tracker)
				}
			}

			// A11: Destructive Migrations
			if isUp && isRuleActive(cfg, a11_destructive_migration.RuleCode) {
				for _, issue := range a11_destructive_migration.CheckMigration(file, content, fileDm) {
					addMigrationIssue(issue, "ARGUS-A11", rootDir, tracker)
				}
			}

			// A15: Forbidden DDL App Role Grants
			if isRuleActive(cfg, a15_ddl_grant.RuleCode) {
				for _, issue := range a15_ddl_grant.CheckMigration(file, content, fileDm, reg) {
					addMigrationIssue(issue, "ARGUS-A15", rootDir, tracker)
				}
			}

			// A27: Non-concurrent Index Creation
			if isRuleActive(cfg, a27_concurrent_index.RuleCode) {
				for _, issue := range a27_concurrent_index.CheckMigration(file, content, fileDm) {
					addMigrationIssue(issue, "ARGUS-A27", rootDir, tracker)
				}
			}

			// A28: Table Locking Constraint Addition
			if isRuleActive(cfg, a28_constraint_lock.RuleCode) {
				for _, issue := range a28_constraint_lock.CheckMigration(file, content, fileDm) {
					addMigrationIssue(issue, "ARGUS-A28", rootDir, tracker)
				}
			}

			// A30: Timestamp without Timezone
			if isRuleActive(cfg, a30_timestamptz.RuleCode) {
				for _, issue := range a30_timestamptz.CheckMigration(file, content, fileDm) {
					addMigrationIssue(issue, "ARGUS-A30", rootDir, tracker)
				}
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
