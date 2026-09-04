// Package updater provides self-updating capabilities for the Argus binary.
package updater

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// hintCacheDuration defines how long to cache the update check result.
	hintCacheDuration = 24 * time.Hour

	// hintTimeout is the maximum time to wait for the GitHub API during hint check.
	hintTimeout = 3 * time.Second

	// hintCacheFile is the basename of the cache file stored in the user's temp directory.
	hintCacheFile = "argus-update-hint-cache"
)

// PrintUpdateHint checks if a newer Argus version exists on GitHub and prints
// a hint to stderr if so. It is designed to be non-blocking and best-effort:
//   - Network failures are silently ignored (no disruption to scan output).
//   - Results are cached for 24 hours to avoid excessive API calls.
//   - The hint is only shown when the current version is a valid release build.
func PrintUpdateHint(currentVersion string) {
	curr := normalizeVersion(currentVersion)
	if curr == "" || curr == "dev" || curr == "unknown" {
		return
	}

	cachePath := hintCachePath()
	if latest, ok := readHintCache(cachePath); ok {
		if isNewer(latest, curr) {
			printHint(latest)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), hintTimeout)
	defer cancel()

	latestTag, err := FetchLatestTag(ctx, http.DefaultClient, apiURL)
	if err != nil {
		return
	}

	latest := normalizeVersion(latestTag)
	writeHintCache(cachePath, latest)

	if isNewer(latest, curr) {
		printHint(latestTag)
	}
}

// printHint emits a styled update notification to stderr.
func printHint(latestTag string) {
	fmt.Fprintf(os.Stderr,
		"\n\033[33m╭─────────────────────────────────────────────╮\033[0m\n"+
			"\033[33m│\033[0m  A new version of Argus is available: \033[1;32m%s\033[0m  \033[33m│\033[0m\n"+
			"\033[33m│\033[0m  Run \033[1;36margus update\033[0m to upgrade               \033[33m│\033[0m\n"+
			"\033[33m╰─────────────────────────────────────────────╯\033[0m\n",
		padVersion(latestTag, 6),
	)
}

// padVersion pads or truncates the version string to a fixed width for
// consistent box alignment.
func padVersion(v string, width int) string {
	if len(v) >= width {
		return v
	}
	return v + strings.Repeat(" ", width-len(v))
}

// hintCachePath returns the path to the hint cache file in the user's temp directory.
func hintCachePath() string {
	return filepath.Join(os.TempDir(), hintCacheFile)
}

// readHintCache reads the cached latest version tag. Returns the tag and true
// if the cache exists and is younger than hintCacheDuration.
func readHintCache(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	if time.Since(info.ModTime()) > hintCacheDuration {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	tag := strings.TrimSpace(string(data))
	if tag == "" {
		return "", false
	}
	return tag, true
}

// writeHintCache writes the latest version tag to the cache file.
// Errors are silently ignored because this is a best-effort optimization.
func writeHintCache(path, version string) {
	_ = os.WriteFile(path, []byte(version), 0o600)
}

// normalizeVersion strips leading "v" and whitespace from a version string.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// isNewer returns true if latest is a strictly newer semantic version than current.
// Both must be dot-separated numeric strings (e.g. "1.2.3"). If either is malformed,
// the function returns false to avoid false-positive hints.
func isNewer(latest, current string) bool {
	lp := strings.Split(latest, ".")
	cp := strings.Split(current, ".")

	maxLen := len(lp)
	if len(cp) > maxLen {
		maxLen = len(cp)
	}

	for i := 0; i < maxLen; i++ {
		var lv, cv int
		if i < len(lp) {
			n, err := strconv.Atoi(lp[i])
			if err != nil {
				return false
			}
			lv = n
		}
		if i < len(cp) {
			n, err := strconv.Atoi(cp[i])
			if err != nil {
				return false
			}
			cv = n
		}
		if lv > cv {
			return true
		}
		if lv < cv {
			return false
		}
	}
	return false
}
