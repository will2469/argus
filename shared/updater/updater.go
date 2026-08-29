// Package updater provides self-updating capabilities for the Argus binary.
package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	repoOwner = "will2469"
	repoName  = "argus"
	apiURL    = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// CheckAndApplyUpdate checks for the latest release and applies it if a newer version is available.
func CheckAndApplyUpdate(currentVersion string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("==> Checking for latest Argus release...")
	latestTag, err := FetchLatestTag(ctx, http.DefaultClient, apiURL)
	if err != nil {
		return fmt.Errorf("could not check for updates: %w", err)
	}

	currNorm := strings.TrimPrefix(strings.TrimSpace(currentVersion), "v")
	latestNorm := strings.TrimPrefix(strings.TrimSpace(latestTag), "v")

	if currNorm == latestNorm && currNorm != "" && currNorm != "dev" {
		fmt.Printf("✓ Argus is already up to date (%s)\n", latestTag)
		return nil
	}

	if currNorm == "dev" {
		fmt.Printf("==> Current build is dev. Upgrading to latest official release (%s)...\n", latestTag)
	} else {
		fmt.Printf("==> Found newer version: %s (current: %s)\n", latestTag, currentVersion)
	}

	ext := "tar.gz"
	binName := "argus"
	if runtime.GOOS == "windows" {
		ext = "zip"
		binName = "argus.exe"
	}

	archiveName := fmt.Sprintf("argus_%s_%s_%s.%s", latestTag, runtime.GOOS, runtime.GOARCH, ext)
	downloadURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", repoOwner, repoName, latestTag, archiveName)

	fmt.Printf("==> Downloading %s...\n", downloadURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d for %s", resp.StatusCode, downloadURL)
	}

	archiveData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read archive payload: %w", err)
	}

	var binaryBytes []byte
	if ext == "zip" {
		binaryBytes, err = extractFromZip(archiveData, binName)
	} else {
		binaryBytes, err = extractFromTarGz(archiveData, binName)
	}
	if err != nil {
		return fmt.Errorf("failed to extract binary from archive: %w", err)
	}

	if err := replaceExecutable(binaryBytes); err != nil {
		return err
	}

	fmt.Printf("✓ Successfully updated Argus to %s!\n", latestTag)
	return nil
}

// FetchLatestTag queries the GitHub API for the latest release tag name.
func FetchLatestTag(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", fmt.Errorf("failed to decode GitHub API response: %w", err)
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("empty tag_name received from GitHub API")
	}
	return rel.TagName, nil
}

func extractFromTarGz(data []byte, targetName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(header.Name) == targetName && header.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("binary %s not found in tar.gz archive", targetName)
}

func extractFromZip(data []byte, targetName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) == targetName {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("binary %s not found in zip archive", targetName)
}

func replaceExecutable(newBinary []byte) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate current executable path: %w", err)
	}
	if realPath, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = realPath
	}

	dir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(dir, "argus-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary update file in %s (try running with sudo?): %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(newBinary); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write update binary: %w", err)
	}
	if err := tmpFile.Chmod(0755); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to mark binary as executable: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close update file: %w", err)
	}

	if runtime.GOOS == "windows" {
		oldPath := execPath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(execPath, oldPath); err != nil {
			return fmt.Errorf("failed to stage existing Windows binary for replacement: %w", err)
		}
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("failed to replace %s (permission denied? Try running with sudo): %w", execPath, err)
	}
	return nil
}
