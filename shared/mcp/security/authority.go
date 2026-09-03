package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PathAuthority enforces strict filesystem isolation boundaries against explicitly
// allowed roots. It defends against both directory traversal ("../") and symlink escapes.
type PathAuthority struct {
	canonicalRoots []string
}

// NewPathAuthority initializes a PathAuthority with canonicalized root directories.
// If roots is empty, it safely defaults to the current working directory (".").
func NewPathAuthority(roots ...string) (*PathAuthority, error) {
	if len(roots) == 0 {
		roots = []string{"."}
	}

	canonical := make([]string, 0, len(roots))
	for _, root := range roots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve root %q: %w", root, err)
		}
		realRoot, err := filepath.EvalSymlinks(abs)
		if err != nil {
			realRoot = resolveSymlinkAncestor(abs)
		}
		canonical = append(canonical, filepath.Clean(realRoot))
	}

	if len(canonical) == 0 {
		return nil, fmt.Errorf("no valid allowed roots configured")
	}

	return &PathAuthority{canonicalRoots: canonical}, nil
}

// CanonicalRoots returns the evaluated physical roots for debugging or logging.
func (pa *PathAuthority) CanonicalRoots() []string {
	return append([]string(nil), pa.canonicalRoots...)
}

// ValidatePath verifies that targetPath resolves strictly within at least one
// of the allowed roots, fully evaluating symlinks and ancestor chains.
func (pa *PathAuthority) ValidatePath(targetPath string) (string, error) {
	trimmed := strings.TrimSpace(targetPath)
	if trimmed == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	absTarget, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %q: %w", targetPath, err)
	}

	// Critical Defense: EvalSymlinks resolves any symlink hops to their true physical target!
	realTarget, err := filepath.EvalSymlinks(absTarget)
	if err != nil {
		// Target might not exist on disk yet (e.g. proposed migration file).
		// Resolve symlinks on the deepest existing ancestor to prevent symlink-nested escapes.
		realTarget = resolveSymlinkAncestor(absTarget)
	}
	realTarget = filepath.Clean(realTarget)

	for _, root := range pa.canonicalRoots {
		if isWithin(root, realTarget) {
			return realTarget, nil
		}
	}

	return "", fmt.Errorf("path authority violation: %q (resolved to %q) is not within allowed roots %v",
		targetPath, realTarget, pa.canonicalRoots)
}

func resolveSymlinkAncestor(path string) string {
	cleaned := filepath.Clean(path)
	var unexistingParts []string

	curr := cleaned
	for {
		realCurr, err := filepath.EvalSymlinks(curr)
		if err == nil {
			result := realCurr
			for i := len(unexistingParts) - 1; i >= 0; i-- {
				result = filepath.Join(result, unexistingParts[i])
			}
			return filepath.Clean(result)
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			return cleaned
		}
		unexistingParts = append(unexistingParts, filepath.Base(curr))
		curr = parent
	}
}

func isWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}
