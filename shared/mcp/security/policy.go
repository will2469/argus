package security

import "fmt"

const (
	MaxMigrationSQLBytes  = 1024 * 1024 // 1MB
	MaxReportTitleChars   = 250
	MaxReportPayloadBytes = 512 * 1024 // 512KB
	MaxScanDirsLimit      = 50
)

// ValidatePathConfinement ensures targetPath does not escape rootDir via
// path traversal ("../"), symlink tricks, or external absolute paths.
// It delegates to PathAuthority for canonical symlink resolution.
func ValidatePathConfinement(rootDir, targetPath string) error {
	authority, err := NewPathAuthority(rootDir)
	if err != nil {
		return fmt.Errorf("failed to create path authority for %q: %w", rootDir, err)
	}
	_, err = authority.ValidatePath(targetPath)
	return err
}
