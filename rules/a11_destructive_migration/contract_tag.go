// Package a11_destructive_migration provides validation for contract-phase annotations and ignore directives.
package a11_destructive_migration

import (
	"strings"

	"github.com/will2469/argus/shared/directives"
)

// IsContractTagged checks if the statement at the given line has a valid contract phase tag.
func IsContractTagged(filename, content string, line int, dm *directives.DirectiveMap) bool {
	if dm != nil && dm.IsLineIgnored(filename, line, RuleCode) {
		return true
	}

	lines := strings.Split(content, "\n")
	if line <= 0 || line > len(lines) {
		return false
	}

	// Check preceding 2 lines for contract tag
	start := line - 2
	if start < 0 {
		start = 0
	}

	for i := start; i < line; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "--") {
			comment := strings.TrimSpace(strings.TrimPrefix(trimmed, "--"))
			if strings.HasPrefix(comment, "argus:contract") {
				parts := strings.Fields(comment)
				return len(parts) >= 2 // requires release tag or reason
			}
		}
	}

	return false
}
