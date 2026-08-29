package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Uninstall removes the current Argus executable from the user's system.
func Uninstall() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}
	if realPath, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = realPath
	}

	return uninstallFile(execPath)
}

func uninstallFile(execPath string) error {
	fmt.Printf("==> Uninstalling Argus from %s...\n", execPath)

	if runtime.GOOS == "windows" {
		oldPath := execPath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(execPath, oldPath); err != nil {
			return fmt.Errorf("failed to stage Windows binary for removal: %w", err)
		}
		execPath = oldPath
	}

	if err := os.Remove(execPath); err != nil {
		return fmt.Errorf("permission denied while removing %s (try running with sudo?): %w", execPath, err)
	}

	fmt.Println("✓ Argus has been successfully uninstalled from your system.")
	return nil
}
