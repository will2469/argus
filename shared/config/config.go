// Package config provides configuration parsing and defaults for Argus.
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"gopkg.in/yaml.v3"
)

// Config represents the root configuration of .argus.yaml.
type Config struct {
	Version       string                `yaml:"version"`
	Options       OptionsConfig         `yaml:"options"`
	ScanDirs      []string              `yaml:"scan_dirs"`      // Optional root-level alias
	MigrationDirs []string              `yaml:"migration_dirs"` // Optional root-level alias
	Rules         map[string]RuleConfig `yaml:"rules"`
}

// OptionsConfig defines global scanner options.
type OptionsConfig struct {
	ReportFormat  string   `yaml:"report_format"`  // "text", "json", "markdown"
	ReportFile    string   `yaml:"report_file"`    // Path to output report markdown/json
	FailOn        string   `yaml:"fail_on"`        // "CRITICAL", "HIGH", "MEDIUM", "LOW"
	ScanDirs      []string `yaml:"scan_dirs"`      // Directories/files to scan for Go code
	MigrationDirs []string `yaml:"migration_dirs"` // Directories containing SQL migrations
	Telemetry     *bool    `yaml:"telemetry"`      // Controls external issue reporting (default: true)
	StrictMode    *bool    `yaml:"strict_mode"`    // Controls failure on parse errors (default: true)
}

// RuleConfig defines parameters for an individual rule.
// Specific rule options are dynamically parsed into Options via yaml:",inline".
type RuleConfig struct {
	Enabled bool                   `yaml:"enabled"`
	Options map[string]interface{} `yaml:",inline"`
}

// DefaultConfig returns safe, sensible defaults when .argus.yaml is not present.
func DefaultConfig() *Config {
	defaultTelemetry := true
	cfg := &Config{
		Version: "1",
		Options: OptionsConfig{
			ReportFormat:  "text",
			FailOn:        "HIGH",
			ScanDirs:      []string{"."},
			MigrationDirs: []string{"migrations"},
			Telemetry:     &defaultTelemetry,
			StrictMode:    &defaultTelemetry, // default: true
		},
		Rules: make(map[string]RuleConfig),
	}

	// Enable all standard 30 rules by default
	for i := 1; i <= 30; i++ {
		code := formatRuleCode(i)
		cfg.Rules[code] = RuleConfig{
			Enabled: true,
			Options: make(map[string]interface{}),
		}
	}

	return cfg
}

// IsTelemetryEnabled checks whether issue reporting / telemetry is enabled.
// ARGUS_TELEMETRY environment variable takes precedence over .argus.yaml.
func (c *Config) IsTelemetryEnabled() bool {
	if env := os.Getenv("ARGUS_TELEMETRY"); env != "" {
		norm := strings.ToLower(strings.TrimSpace(env))
		if norm == "false" || norm == "0" || norm == "off" || norm == "no" {
			return false
		}
		if norm == "true" || norm == "1" || norm == "on" || norm == "yes" {
			return true
		}
	}
	if c != nil && c.Options.Telemetry != nil {
		return *c.Options.Telemetry
	}
	return true
}

// IsStrictMode checks whether strict parsing mode is enabled.
// In strict mode, unparseable migrations fail the audit; in permissive mode, they are logged as warnings.
// ARGUS_STRICT environment variable takes precedence over .argus.yaml.
func (c *Config) IsStrictMode() bool {
	if env := os.Getenv("ARGUS_STRICT"); env != "" {
		norm := strings.ToLower(strings.TrimSpace(env))
		if norm == "false" || norm == "0" || norm == "off" || norm == "no" {
			return false
		}
		if norm == "true" || norm == "1" || norm == "on" || norm == "yes" {
			return true
		}
	}
	if c != nil && c.Options.StrictMode != nil {
		return *c.Options.StrictMode
	}
	return true
}

func formatRuleCode(i int) string {
	if i < 10 {
		return "ARGUS-A0" + string(rune('0'+i))
	}
	return "ARGUS-A" + string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// LoadConfig attempts to read .argus.yaml from rootDir or any parent directory.
func LoadConfig(startDir string) (*Config, error) {
	configPath := findConfigFile(startDir)
	if configPath == "" {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return DefaultConfig(), nil
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), err
	}
	return cfg, nil
}

func findConfigFile(dir string) string {
	curr := dir
	for {
		candidate := filepath.Join(curr, ".argus.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return ""
}

// GetScanDirs returns configured scan directories, or defaults to ["."] if not specified.
func (c *Config) GetScanDirs() []string {
	if c != nil {
		if len(c.Options.ScanDirs) > 0 {
			return c.Options.ScanDirs
		}
		if len(c.ScanDirs) > 0 {
			return c.ScanDirs
		}
	}
	return []string{"."}
}

// GetMigrationDirs returns configured migration directories, or defaults to ["migrations"] if not specified.
func (c *Config) GetMigrationDirs() []string {
	if c != nil {
		if len(c.Options.MigrationDirs) > 0 {
			return c.Options.MigrationDirs
		}
		if len(c.MigrationDirs) > 0 {
			return c.MigrationDirs
		}
	}
	return []string{"migrations"}
}

// FindMatchingMigrationDir checks if the current package directory hosts or contains any configured migration directory.
func (c *Config) FindMatchingMigrationDir(pkgDir string) string {
	if pkgDir == "" {
		return ""
	}

	// 1. Direct local "migrations" child directory
	localMig := filepath.Join(pkgDir, "migrations")
	if info, err := os.Stat(localMig); err == nil && info.IsDir() {
		return localMig
	}

	// 2. Configured migration dirs (match against package path or relative child)
	normalizedPkgDir := filepath.ToSlash(pkgDir)
	for _, mDir := range c.GetMigrationDirs() {
		normalizedMDir := filepath.ToSlash(mDir)

		// Case A: pkgDir ends with the parent directory of mDir (e.g. pkgDir=.../pkg/argus and mDir=pkg/argus/migrations)
		parentOfMDir := filepath.ToSlash(filepath.Dir(normalizedMDir))
		if parentOfMDir != "." && strings.HasSuffix(normalizedPkgDir, parentOfMDir) {
			candidate := filepath.Join(pkgDir, filepath.Base(normalizedMDir))
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}

		// Case B: candidate directly under pkgDir
		candidate := filepath.Join(pkgDir, filepath.FromSlash(mDir))
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

// IsRuleEnabled checks if a rule is active.
func (c *Config) IsRuleEnabled(code string) bool {
	if c == nil || c.Rules == nil {
		return true
	}
	r, exists := c.Rules[code]
	if !exists {
		return true
	}
	return r.Enabled
}

// GetStringSlice retrieves a slice of strings option for a rule with fallback to defaultVal.
func (c *Config) GetStringSlice(code, key string, defaultVal []string) []string {
	if c == nil || c.Rules == nil {
		return defaultVal
	}
	r, exists := c.Rules[code]
	if !exists || r.Options == nil {
		return defaultVal
	}
	raw, ok := r.Options[key]
	if !ok || raw == nil {
		return defaultVal
	}

	switch val := raw.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return defaultVal
}

// GetString retrieves a string option for a rule with fallback to defaultVal.
func (c *Config) GetString(code, key string, defaultVal string) string {
	if c == nil || c.Rules == nil {
		return defaultVal
	}
	r, exists := c.Rules[code]
	if !exists || r.Options == nil {
		return defaultVal
	}
	raw, ok := r.Options[key]
	if !ok || raw == nil {
		return defaultVal
	}
	if s, ok := raw.(string); ok {
		return s
	}
	return defaultVal
}

// GetInt retrieves an integer option for a rule with fallback to defaultVal.
func (c *Config) GetInt(code, key string, defaultVal int) int {
	if c == nil || c.Rules == nil {
		return defaultVal
	}
	r, exists := c.Rules[code]
	if !exists || r.Options == nil {
		return defaultVal
	}
	raw, ok := r.Options[key]
	if !ok || raw == nil {
		return defaultVal
	}

	switch val := raw.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		if n, err := strconv.Atoi(val); err == nil {
			return n
		}
	}
	return defaultVal
}

// GetBool retrieves a boolean option for a rule with fallback to defaultVal.
func (c *Config) GetBool(code, key string, defaultVal bool) bool {
	if c == nil || c.Rules == nil {
		return defaultVal
	}
	r, exists := c.Rules[code]
	if !exists || r.Options == nil {
		return defaultVal
	}
	raw, ok := r.Options[key]
	if !ok || raw == nil {
		return defaultVal
	}
	if b, ok := raw.(bool); ok {
		return b
	}
	return defaultVal
}

// Analyzer provides the Config instance to downstream analyzers.
var Analyzer = &analysis.Analyzer{
	Name:       "argus_config",
	Doc:        "Loads .argus.yaml configuration for Argus analyzers with fallback defaults",
	Run:        runConfigAnalyzer,
	ResultType: reflect.TypeOf((*Config)(nil)),
}

func runConfigAnalyzer(pass *analysis.Pass) (interface{}, error) {
	dir := "."
	if len(pass.Files) > 0 {
		pos := pass.Fset.Position(pass.Files[0].Package)
		if pos.Filename != "" {
			dir = filepath.Dir(pos.Filename)
		}
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		return DefaultConfig(), nil
	}
	return cfg, nil
}
