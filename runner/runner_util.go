package runner

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func findFilesWithExt(root, ext string) []string {
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, ext) {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func unquoteStringLit(raw string) string {
	if unquoted, err := strconv.Unquote(raw); err == nil {
		return unquoted
	}
	return strings.Trim(raw, "`\"")
}
