package version

import (
	"strings"
	"testing"
)

func TestVersion_Get(t *testing.T) {
	orig := Version
	defer func() { Version = orig }()

	// 1. Explicit version with 'v' prefix
	Version = "v1.2.3"
	if got := Get(); got != "1.2.3" {
		t.Fatalf("expected 1.2.3, got: %s", got)
	}

	// 2. Dev fallback to DefaultVersion
	Version = "dev"
	if got := Get(); got != DefaultVersion {
		t.Fatalf("expected %s, got: %s", DefaultVersion, got)
	}

	// 3. Empty fallback to DefaultVersion
	Version = ""
	if got := Get(); got != DefaultVersion {
		t.Fatalf("expected %s, got: %s", DefaultVersion, got)
	}
}

func TestVersion_FullInfo(t *testing.T) {
	origV, origC, origD := Version, Commit, Date
	defer func() {
		Version = origV
		Commit = origC
		Date = origD
	}()

	Version = "1.0.0"
	Commit = "abcdef1"
	Date = "2026-09-04"

	info := FullInfo()
	if !strings.Contains(info, "1.0.0") || !strings.Contains(info, "abcdef1") || !strings.Contains(info, "2026-09-04") {
		t.Fatalf("unexpected FullInfo: %s", info)
	}
}
