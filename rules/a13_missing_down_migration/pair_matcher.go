// Package a13_missing_down_migration validates that every .up.sql migration has a corresponding .down.sql file.
package a13_missing_down_migration

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// MatchPairExistence checks if a .up.sql migration has a corresponding .down.sql file in the directory map.
func MatchPairExistence(dir, upName string, existingFiles map[string]bool, dm *directives.DirectiveMap) (string, *migration.Issue) {
	downName := strings.TrimSuffix(upName, ".up.sql") + ".down.sql"
	upPath := filepath.Join(dir, upName)
	downPath := filepath.Join(dir, downName)

	if !existingFiles[downName] {
		if dm != nil && dm.IsLineIgnored(upPath, 1, RuleCode) {
			return downPath, nil
		}
		return downPath, &migration.Issue{
			Rule:     RuleCode,
			Filename: upPath,
			Line:     1,
			Message:  fmt.Sprintf("Missing required rollback file %q for migration %q", downName, upName),
			Severity: "CRITICAL",
		}
	}

	return downPath, nil
}
