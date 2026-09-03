package security

import (
	"fmt"
	"os"
)

// CheckPathConfinement performs advisory path validation only.
// It ensures targetPath does not escape rootDir via path traversal ("../"),
// symlink tricks, or external absolute paths, delegating to PathAuthority
// for canonical symlink resolution.
//
// WARNING: This function MUST NOT be used as authorization for a subsequent
// filesystem operation. It is advisory-only and vulnerable to TOCTOU races.
// Use SafeOpen or SafeStat when the validated path will actually be accessed,
// as they provide kernel-enforced containment via os.Root (openat2 RESOLVE_BENEATH).
func CheckPathConfinement(rootDir, targetPath string) error {
	authority, err := NewPathAuthority(rootDir)
	if err != nil {
		return fmt.Errorf("failed to create path authority for %q: %w", rootDir, err)
	}
	_, err = authority.ValidatePath(targetPath)
	return err
}

// SafeOpenDir atomically opens a file within a single root directory,
// immune to TOCTOU symlink races. It creates a PathAuthority for the
// root and uses SafeOpen for kernel-enforced containment.
//
// This is a convenience wrapper for single-root operations. For multi-root
// scenarios, create a PathAuthority directly and use its SafeOpen method.
//
// The returned *os.File is opened for reading only. The caller is responsible
// for closing the file.
func SafeOpenDir(rootDir, targetPath string) (*os.File, error) {
	authority, err := NewPathAuthority(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create path authority for %q: %w", rootDir, err)
	}
	return authority.SafeOpen(targetPath)
}

// SafeStatDir atomically stats a file within a single root directory,
// immune to TOCTOU symlink races. It creates a PathAuthority for the
// root and uses SafeStat for kernel-enforced containment.
//
// This is a convenience wrapper for single-root operations. For multi-root
// scenarios, create a PathAuthority directly and use its SafeStat method.
func SafeStatDir(rootDir, targetPath string) (os.FileInfo, error) {
	authority, err := NewPathAuthority(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create path authority for %q: %w", rootDir, err)
	}
	return authority.SafeStat(targetPath)
}
