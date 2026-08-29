// Package migration provides shared types and utilities for migration SQL file scanners.
package migration

import (
	"strings"
)

// Issue represents a diagnostic finding in a SQL migration file.
type Issue struct {
	Rule     string
	Filename string
	Line     int
	Message  string
	Severity string // "CRITICAL", "HIGH", "MEDIUM", "LOW"
}

// FindLineFromOffset computes the 1-based line number corresponding to a byte offset in content.
func FindLineFromOffset(content string, offset int) int {
	if offset <= 0 {
		return 1
	}
	if offset > len(content) {
		offset = len(content)
	}
	return strings.Count(content[:offset], "\n") + 1
}

// FindLineForKeyword searches for the 1-based line of a case-insensitive substring in content.
func FindLineForKeyword(content, keyword string) int {
	idx := strings.Index(strings.ToLower(content), strings.ToLower(keyword))
	if idx == -1 {
		return 1
	}
	return strings.Count(content[:idx], "\n") + 1
}
