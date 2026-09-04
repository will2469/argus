package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name    string
		latest  string
		current string
		want    bool
	}{
		{"newer major", "2.0.0", "1.0.0", true},
		{"newer minor", "1.1.0", "1.0.0", true},
		{"newer patch", "1.0.1", "1.0.0", true},
		{"same version", "1.0.0", "1.0.0", false},
		{"older major", "1.0.0", "2.0.0", false},
		{"older minor", "1.0.0", "1.1.0", false},
		{"older patch", "1.0.0", "1.0.1", false},
		{"different segment lengths latest", "1.0.0.1", "1.0.0", true},
		{"different segment lengths current", "1.0.0", "1.0.0.1", false},
		{"malformed latest", "abc", "1.0.0", false},
		{"malformed current", "1.0.0", "abc", false},
		{"both malformed", "x", "y", false},
		{"empty latest", "", "1.0.0", false},
		{"multi digit", "1.12.3", "1.9.3", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNewer(tt.latest, tt.current)
			if got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"  v1.0.0 ", "1.0.0"},
		{"dev", "dev"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeVersion(tt.input)
			if got != tt.want {
				t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHintCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "argus-test-cache")

	// Initially no cache.
	_, ok := readHintCache(path)
	if ok {
		t.Fatal("expected no cache initially")
	}

	// Write and read back.
	writeHintCache(path, "1.2.3")
	tag, ok := readHintCache(path)
	if !ok {
		t.Fatal("expected cache to be readable")
	}
	if tag != "1.2.3" {
		t.Fatalf("expected cached tag 1.2.3, got %s", tag)
	}

	// Expire the cache by backdating ModTime.
	expired := time.Now().Add(-(hintCacheDuration + time.Minute))
	if err := os.Chtimes(path, expired, expired); err != nil {
		t.Fatalf("failed to backdate cache: %v", err)
	}
	_, ok = readHintCache(path)
	if ok {
		t.Fatal("expected expired cache to be rejected")
	}
}

func TestPrintUpdateHintNewerVersion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v2.0.0","html_url":"https://github.com/will2469/argus/releases/tag/v2.0.0"}`))
	}))
	defer ts.Close()

	// Clear any existing cache.
	cachePath := hintCachePath()
	_ = os.Remove(cachePath)
	defer os.Remove(cachePath)

	// Verify FetchLatestTag works with mock server (integration sanity check).
	tag, err := FetchLatestTag(context.Background(), ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("FetchLatestTag failed: %v", err)
	}
	if tag != "v2.0.0" {
		t.Fatalf("expected v2.0.0, got %s", tag)
	}

	// Verify comparison logic.
	if !isNewer(normalizeVersion(tag), normalizeVersion("v1.0.0")) {
		t.Fatal("expected v2.0.0 to be newer than v1.0.0")
	}
}

func TestPrintUpdateHintDevVersion(t *testing.T) {
	// Dev builds should silently skip hint check.
	// This just ensures no panic or unexpected behavior.
	PrintUpdateHint("dev")
	PrintUpdateHint("")
	PrintUpdateHint("unknown")
}

func TestPadVersion(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  string
	}{
		{"v1.0.0", 6, "v1.0.0"},
		{"v1.0", 6, "v1.0  "},
		{"v1.0.0.1", 6, "v1.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := padVersion(tt.input, tt.width)
			if got != tt.want {
				t.Errorf("padVersion(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
			}
		})
	}
}
