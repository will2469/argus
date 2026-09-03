package security

import (
	"fmt"
	"os"
)

// SafeOpen atomically opens a file within one of the PathAuthority's allowed roots,
// immune to TOCTOU symlink races. It leverages os.Root (backed by openat2 with
// RESOLVE_BENEATH on Linux) to ensure the kernel itself enforces containment.
//
// The returned *os.File is opened for reading only. The caller is responsible for
// closing the file.
//
// If the target path escapes all allowed roots (including via symlink swap between
// check and open), SafeOpen returns an error.
func (pa *PathAuthority) SafeOpen(targetPath string) (*os.File, error) {
	for _, root := range pa.canonicalRoots {
		f, err := openBeneath(root, targetPath)
		if err == nil {
			return f, nil
		}
	}
	return nil, fmt.Errorf("safe open violation: %q is not within allowed roots %v", targetPath, pa.canonicalRoots)
}

// SafeStat atomically stats a file within one of the PathAuthority's allowed roots,
// immune to TOCTOU symlink races.
func (pa *PathAuthority) SafeStat(targetPath string) (os.FileInfo, error) {
	for _, root := range pa.canonicalRoots {
		info, err := statBeneath(root, targetPath)
		if err == nil {
			return info, nil
		}
	}
	return nil, fmt.Errorf("safe stat violation: %q is not within allowed roots %v", targetPath, pa.canonicalRoots)
}

// openBeneath opens targetPath relative to rootDir using os.Root, which provides
// kernel-level containment via openat2(RESOLVE_BENEATH) on Linux.
func openBeneath(rootDir, targetPath string) (*os.File, error) {
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open root %q: %w", rootDir, err)
	}
	defer root.Close()

	return root.Open(targetPath)
}

// statBeneath stats targetPath relative to rootDir using os.Root.
func statBeneath(rootDir, targetPath string) (os.FileInfo, error) {
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open root %q: %w", rootDir, err)
	}
	defer root.Close()

	return root.Stat(targetPath)
}
