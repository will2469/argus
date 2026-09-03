package runner

import (
	"io/fs"
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

func findFilesWithExtFS(fsys fs.FS, root, ext string) []string {
	var files []string
	cleanRoot := filepath.ToSlash(filepath.Clean(root))
	if cleanRoot == "" {
		cleanRoot = "."
	}
	_ = fs.WalkDir(fsys, cleanRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(path, ext) {
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
