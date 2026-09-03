package security

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// RootCapability holds an active *os.Root descriptor providing kernel-enforced
// filesystem containment (openat2 with RESOLVE_BENEATH on Linux).
type RootCapability struct {
	root   *os.Root
	fsys   fs.FS
	closed bool
}

// FS returns the confined fs.FS rooted at the opened directory.
func (rc *RootCapability) FS() fs.FS {
	if rc == nil {
		return nil
	}
	return rc.fsys
}

// Close closes the underlying root descriptor.
func (rc *RootCapability) Close() error {
	if rc == nil || rc.closed || rc.root == nil {
		return nil
	}
	rc.closed = true
	return rc.root.Close()
}

// PrimaryRoot returns the primary canonical root directory.
func (pa *PathAuthority) PrimaryRoot() string {
	if len(pa.canonicalRoots) == 0 {
		return "."
	}
	return pa.canonicalRoots[0]
}

// AuthorizeAndOpen validates input paths and opens an atomic *os.Root capability
// bound to the primary allowed root.
func (pa *PathAuthority) AuthorizeAndOpen(dirs, migrations []string) (*RootCapability, []string, []string, error) {
	primary := pa.PrimaryRoot()
	rootHandle, err := os.OpenRoot(primary)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to open root capability for %q: %w", primary, err)
	}

	cleanDirs, err := pa.normalizeToRoot(primary, dirs)
	if err != nil {
		_ = rootHandle.Close()
		return nil, nil, nil, err
	}

	cleanMigrations, err := pa.normalizeToRoot(primary, migrations)
	if err != nil {
		_ = rootHandle.Close()
		return nil, nil, nil, err
	}

	cap := &RootCapability{
		root: rootHandle,
		fsys: rootHandle.FS(),
	}
	return cap, cleanDirs, cleanMigrations, nil
}

func (pa *PathAuthority) normalizeToRoot(root string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	result := make([]string, 0, len(paths))
	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" || trimmed == "." {
			result = append(result, ".")
			continue
		}

		var absTarget string
		if filepath.IsAbs(trimmed) {
			absTarget = filepath.Clean(trimmed)
		} else {
			absTarget = filepath.Clean(filepath.Join(root, trimmed))
		}

		if !isWithin(root, absTarget) {
			return nil, fmt.Errorf("path authority violation: %q is not within allowed root %q", p, root)
		}

		rel, err := filepath.Rel(root, absTarget)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("path authority violation: %q escapes allowed root %q", p, root)
		}

		cleanRel := filepath.ToSlash(filepath.Clean(rel))
		result = append(result, cleanRel)
	}
	return result, nil
}
