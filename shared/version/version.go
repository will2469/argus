// Package version provides centralized semantic versioning and build metadata for Argus.
package version

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// DefaultVersion is the base semantic release of Argus.
const DefaultVersion = "1.2.0"

var (
	// Version can be injected at build-time via -ldflags:
	//   -ldflags "-X github.com/will2469/argus/shared/version.Version=v1.2.3"
	Version = DefaultVersion

	// Commit hash injected at build time.
	Commit = "none"

	// Date timestamp injected at build time.
	Date = "unknown"
)

// Get returns the clean semantic version string (e.g. "1.0.0").
// It resolves in priority order:
// 1. Build-time ldflags (if not "dev" / empty)
// 2. Go runtime debug.ReadBuildInfo() (for `go install` binaries)
// 3. DefaultVersion constant fallback
func Get() string {
	v := strings.TrimSpace(Version)
	if v != "" && v != "dev" && v != "unknown" {
		return strings.TrimPrefix(v, "v")
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return strings.TrimPrefix(info.Main.Version, "v")
		}
	}

	return DefaultVersion
}

// FullInfo returns formatted build metadata suitable for CLI banner output.
func FullInfo() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Get(), Commit, Date)
}
