package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsStandaloneRun(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "empty args", args: []string{}, want: true},
		{name: "help short", args: []string{"-h"}, want: true},
		{name: "help double dash", args: []string{"--help"}, want: true},
		{name: "help single dash", args: []string{"-help"}, want: true},
		{name: "help word", args: []string{"help"}, want: true},
		{name: "version short", args: []string{"-v"}, want: true},
		{name: "version double dash", args: []string{"--version"}, want: true},
		{name: "version single dash", args: []string{"-version"}, want: true},
		{name: "version word", args: []string{"version"}, want: true},
		{name: "update short", args: []string{"-u"}, want: true},
		{name: "update double dash", args: []string{"--update"}, want: true},
		{name: "update single dash", args: []string{"-update"}, want: true},
		{name: "update word", args: []string{"update"}, want: true},
		{name: "upgrade word", args: []string{"upgrade"}, want: true},
		{name: "uninstall word", args: []string{"uninstall"}, want: true},
		{name: "uninstall double dash", args: []string{"--uninstall"}, want: true},
		{name: "uninstall single dash", args: []string{"-uninstall"}, want: true},
		{name: "subcommand check", args: []string{"check"}, want: true},
		{name: "subcommand scan", args: []string{"scan"}, want: true},
		{name: "subcommand audit", args: []string{"audit"}, want: true},
		{name: "subcommand report", args: []string{"report"}, want: true},
		{name: "flag output double dash", args: []string{"--output=report.md"}, want: true},
		{name: "flag output single dash", args: []string{"-output=report.md"}, want: true},
		{name: "flag dirs double dash", args: []string{"--dirs=pkg"}, want: true},
		{name: "flag dirs single dash", args: []string{"-dirs=pkg"}, want: true},
		{name: "flag migrations double dash", args: []string{"--migrations=db"}, want: true},
		{name: "flag migrations single dash", args: []string{"-migrations=db"}, want: true},
		{name: "flag no-report double dash", args: []string{"--no-report"}, want: true},
		{name: "flag no-report single dash", args: []string{"-no-report"}, want: true},
		{name: "file target", args: []string{"./..."}, want: true},
		{name: "vettool flags flag", args: []string{"-flags"}, want: false},
		{name: "vettool V flag", args: []string{"-V"}, want: false},
		{name: "vettool test flag", args: []string{"-test=true"}, want: false},
		{name: "vettool json flag", args: []string{"-json"}, want: false},
		{name: "vettool c flag", args: []string{"-c=2"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStandaloneRun(tt.args)
			if got != tt.want {
				t.Errorf("isStandaloneRun(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestHelpFlagInvariant(t *testing.T) {
	tempDir := t.TempDir()
	binPath := filepath.Join(tempDir, "argus")

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build argus binary: %v, output: %s", err, string(out))
	}

	helpFlags := []string{"-help", "--help", "-h", "help"}
	var baselineOutput string

	for i, flag := range helpFlags {
		cmd := exec.Command(binPath, flag)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("argus %s failed with error: %v, output: %s", flag, err, string(out))
		}
		outputStr := strings.TrimSpace(string(out))

		if i == 0 {
			baselineOutput = outputStr
			if !strings.Contains(baselineOutput, "Usage: argus") {
				t.Fatalf("expected usage output, got: %s", baselineOutput)
			}
		} else {
			if outputStr != baselineOutput {
				t.Errorf("output mismatch for flag %q:\nGot:\n%s\nWant:\n%s", flag, outputStr, baselineOutput)
			}
		}
	}
}

func TestVersionFlagInvariant(t *testing.T) {
	tempDir := t.TempDir()
	binPath := filepath.Join(tempDir, "argus")

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build argus binary: %v, output: %s", err, string(out))
	}

	versionFlags := []string{"-version", "--version", "-v", "version"}
	var baselineOutput string

	for i, flag := range versionFlags {
		cmd := exec.Command(binPath, flag)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("argus %s failed with error: %v, output: %s", flag, err, string(out))
		}
		outputStr := strings.TrimSpace(string(out))

		if i == 0 {
			baselineOutput = outputStr
			if !strings.Contains(baselineOutput, "argus") {
				t.Fatalf("expected version output, got: %s", baselineOutput)
			}
		} else {
			if outputStr != baselineOutput {
				t.Errorf("output mismatch for flag %q:\nGot:\n%s\nWant:\n%s", flag, outputStr, baselineOutput)
			}
		}
	}
}
