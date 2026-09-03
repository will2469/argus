package runner

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/will2469/argus/shared/config"
)

// AuditConfig defines directory scan targets and options.
type AuditConfig struct {
	RootDir       string
	ScanDirs      []string
	MigrationDirs []string
	Config        *config.Config
	Analyzers     []*analysis.Analyzer
	Context       context.Context
	FS            fs.FS
}

// RunAuditWithConfig scans specified targets using provided configuration.
func RunAuditWithConfig(cfg AuditConfig) (*AuditResult, error) {
	tracker := NewMetricsTracker()
	rootDir := cfg.RootDir
	if rootDir == "" {
		rootDir = "."
	}

	appCfg := cfg.Config
	if appCfg == nil {
		loaded, _ := config.LoadConfig(rootDir)
		appCfg = loaded
	}

	// 1. Scan backend Go files
	scanDirs := cfg.ScanDirs
	if len(scanDirs) == 0 {
		scanDirs = appCfg.GetScanDirs()
	}

	var goFiles []string
	seenFiles := make(map[string]bool)
	for _, dir := range scanDirs {
		var files []string
		if cfg.FS != nil {
			cleanTarget := filepath.ToSlash(filepath.Clean(dir))
			if cleanTarget == "" {
				cleanTarget = "."
			}
			if strings.HasSuffix(cleanTarget, ".go") {
				files = []string{cleanTarget}
			} else {
				files = findFilesWithExtFS(cfg.FS, cleanTarget, ".go")
			}
		} else {
			target := dir
			if !filepath.IsAbs(target) {
				target = filepath.Join(rootDir, target)
			}
			if strings.HasSuffix(target, ".go") {
				files = []string{target}
			} else {
				files = findFilesWithExt(target, ".go")
			}
		}
		for _, f := range files {
			if !seenFiles[f] {
				seenFiles[f] = true
				goFiles = append(goFiles, f)
			}
		}
	}

	for _, file := range goFiles {
		if cfg.Context != nil && cfg.Context.Err() != nil {
			return nil, cfg.Context.Err()
		}
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		tracker.IncrementScannedFiles(1)
		scanGoSourceFile(file, rootDir, tracker, appCfg, cfg.FS)
	}

	if cfg.Context != nil && cfg.Context.Err() != nil {
		return nil, cfg.Context.Err()
	}

	// 2. Scan migration SQL files with all migration rules (A11, A13, A15, A27, A28, A29, A30)
	migrationDirs := cfg.MigrationDirs
	if len(migrationDirs) == 0 {
		migrationDirs = appCfg.GetMigrationDirs()
	}

	scanMigrationDirectories(migrationDirs, rootDir, tracker, appCfg, cfg.FS)

	// 3. Build dynamic rule audit info from active analyzers and tracker issues
	rulesInfo := BuildDynamicRuleAuditInfo(cfg.Analyzers, appCfg, tracker.verifiedQuerySites, tracker.scannedMigrationFiles, tracker.scannedFiles, tracker.issues)
	tracker.SetAttachedRules(rulesInfo)

	return tracker.Snapshot(), nil
}

// RunAudit scans default repository targets and produces an AuditResult.
func RunAudit(rootDir string) (*AuditResult, error) {
	return RunAuditWithConfig(AuditConfig{
		RootDir: rootDir,
	})
}
