// Package main provides the dual-mode CLI and vettool entry point for Argus Checker.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/will2469/argus/rules"
	"github.com/will2469/argus/runner"
	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/mcp"
	"github.com/will2469/argus/shared/updater"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if isStandaloneRun(os.Args[1:]) {
		runStandalone()
		return
	}
	multichecker.Main(rules.AllAnalyzers...)
}

func isStandaloneRun(args []string) bool {
	if len(args) == 0 {
		return true
	}
	for _, arg := range args {
		if strings.HasSuffix(arg, ".md") ||
			strings.HasPrefix(arg, "--output") || strings.HasPrefix(arg, "-output") ||
			strings.HasPrefix(arg, "--dirs") || strings.HasPrefix(arg, "-dirs") ||
			strings.HasPrefix(arg, "--migrations") || strings.HasPrefix(arg, "-migrations") ||
			arg == "--no-report" || arg == "-no-report" ||
			arg == "-h" || arg == "--help" || arg == "help" ||
			arg == "-v" || arg == "--version" || arg == "version" ||
			arg == "-u" || arg == "--update" || arg == "update" || arg == "upgrade" ||
			arg == "mcp" || arg == "serve-mcp" ||
			arg == "uninstall" || arg == "--uninstall" ||
			arg == "audit" || arg == "report" {
			return true
		}
		if strings.HasPrefix(arg, "-flags") || strings.HasPrefix(arg, "-V") || strings.HasPrefix(arg, "-test=") {
			return false
		}
	}
	return !strings.HasPrefix(args[0], "-")
}

func runStandalone() {
	rootDir, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding repo root: %v\n", err)
		os.Exit(1)
	}

	var (
		outputFile    string
		noReport      bool
		scanDirs      []string
		migrationDirs []string
	)

	for _, arg := range os.Args[1:] {
		switch {
		case arg == "-v" || arg == "--version" || arg == "version":
			fmt.Printf("argus %s (commit: %s, built: %s)\n", version, commit, date)
			os.Exit(0)
		case arg == "-h" || arg == "--help" || arg == "help":
			printUsage()
			os.Exit(0)
		case arg == "-u" || arg == "--update" || arg == "update" || arg == "upgrade":
			if err := updater.CheckAndApplyUpdate(version); err != nil {
				fmt.Fprintf(os.Stderr, "Error updating Argus: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		case arg == "mcp" || arg == "serve-mcp":
			if err := mcp.ServeStdio(); err != nil {
				fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		case arg == "uninstall" || arg == "--uninstall":
			if err := updater.Uninstall(); err != nil {
				fmt.Fprintf(os.Stderr, "Error uninstalling Argus: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		case arg == "--no-report" || arg == "-no-report":
			noReport = true
		case strings.HasPrefix(arg, "--output="):
			outputFile = strings.TrimPrefix(arg, "--output=")
		case strings.HasPrefix(arg, "-output="):
			outputFile = strings.TrimPrefix(arg, "-output=")
		case strings.HasSuffix(arg, ".md"):
			outputFile = arg
		case strings.HasPrefix(arg, "--dirs="):
			dirs := strings.Split(strings.TrimPrefix(arg, "--dirs="), ",")
			for _, d := range dirs {
				if trimmed := strings.TrimSpace(d); trimmed != "" {
					scanDirs = append(scanDirs, trimmed)
				}
			}
		case strings.HasPrefix(arg, "-dirs="):
			dirs := strings.Split(strings.TrimPrefix(arg, "-dirs="), ",")
			for _, d := range dirs {
				if trimmed := strings.TrimSpace(d); trimmed != "" {
					scanDirs = append(scanDirs, trimmed)
				}
			}
		case strings.HasPrefix(arg, "--migrations="):
			dirs := strings.Split(strings.TrimPrefix(arg, "--migrations="), ",")
			for _, d := range dirs {
				if trimmed := strings.TrimSpace(d); trimmed != "" {
					migrationDirs = append(migrationDirs, trimmed)
				}
			}
		case strings.HasPrefix(arg, "-migrations="):
			dirs := strings.Split(strings.TrimPrefix(arg, "-migrations="), ",")
			for _, d := range dirs {
				if trimmed := strings.TrimSpace(d); trimmed != "" {
					migrationDirs = append(migrationDirs, trimmed)
				}
			}
		case !strings.HasPrefix(arg, "-"):
			scanDirs = append(scanDirs, arg)
		}
	}

	appCfg, _ := config.LoadConfig(rootDir)

	cfg := runner.AuditConfig{
		RootDir:       rootDir,
		ScanDirs:      scanDirs,
		MigrationDirs: migrationDirs,
		Config:        appCfg,
		Analyzers:     rules.AllAnalyzers,
	}

	result, err := runner.RunAuditWithConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Audit error: %v\n", err)
		os.Exit(1)
	}

	if !noReport && outputFile == "" && appCfg != nil && appCfg.Options.ReportFile != "" {
		outputFile = appCfg.Options.ReportFile
	}

	if !noReport && outputFile != "" {
		report := runner.GenerateMarkdownReport(result, rootDir)
		if !filepath.IsAbs(outputFile) {
			outputFile = filepath.Join(rootDir, outputFile)
		}
		outDir := filepath.Dir(outputFile)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output dir: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(outputFile, []byte(report), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing report file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf(" Argus SQL audit report saved to %s\n", outputFile)
	}

	if len(result.Issues) > 0 {
		fmt.Fprintf(os.Stderr, "\n Found %d Argus SQL hygiene violations:\n", len(result.Issues))
		for _, issue := range result.Issues {
			fmt.Fprintf(os.Stderr, "  - %s:%d [%s] %s\n", issue.File, issue.Line, issue.Rule, issue.Message)
		}
		os.Exit(1)
	} else {
		fmt.Printf("\n Argus SQL Hygiene & Anti-Overfetching (No SELECT *) is 100%% clean! (%d query sites, %d parameterized placeholders)\n",
			result.VerifiedQuerySites, result.VerifiedParameterizedSites)
		os.Exit(0)
	}
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return cwd, nil
}

func printUsage() {
	fmt.Println("Usage: argus [options] [directories...]")
	fmt.Println("       argus update")
	fmt.Println("       argus mcp")
	fmt.Println("       argus uninstall")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  update                  Check and update Argus to the latest release")
	fmt.Println("  mcp                     Start MCP (Model Context Protocol) server for AI agents")
	fmt.Println("  uninstall               Remove the Argus binary from your system")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --output=<file.md>      Path to output markdown report")
	fmt.Println("  --dirs=<d1,d2>          Comma-separated list of Go directories/files to scan")
	fmt.Println("  --migrations=<d1,d2>    Comma-separated list of SQL migration directories")
	fmt.Println("  --no-report             Run in memory without generating a report file")
	fmt.Println("  -u, --update            Alias for 'argus update'")
	fmt.Println("  -v, --version           Show version information and exit")
	fmt.Println("  -h, --help              Show this help message")
}
