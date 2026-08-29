// Package a29_unindexed_fk cross-references foreign keys against supporting B-tree index columns.
package a29_unindexed_fk

import (
	"fmt"
	"strings"

	"github.com/will2469/argus/shared/config"
	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
)

// MatchUnindexedForeignKeys finds all FK relationships lacking a leading-column index.
func (g *SchemaGraph) MatchUnindexedForeignKeys(dm *directives.DirectiveMap, cfg *config.Config) []migration.Issue {
	if g == nil {
		return nil
	}

	ignoredPrefixes := []string{"ref_"}
	if cfg != nil {
		ignoredPrefixes = cfg.GetStringSlice(RuleCode, "ignore_parent_prefixes", []string{"ref_"})
	}

	var issues []migration.Issue
	for _, fk := range g.FKs {
		// Check ignored parent table prefixes (e.g. ref_)
		isIgnoredParent := false
		for _, prefix := range ignoredPrefixes {
			if strings.HasPrefix(fk.ParentTable, prefix) {
				isIgnoredParent = true
				break
			}
		}
		if isIgnoredParent {
			continue
		}

		key := fk.Table + "." + fk.Column
		if !g.IndexedCols[key] {
			if dm != nil && dm.IsLineIgnored(fk.Filename, fk.Line, RuleCode) {
				continue
			}
			msg := fmt.Sprintf("Foreign Key on %q column %q (references %q) has no supporting leading-column B-tree index, risking table scan lockups during parent deletes", fk.Table, fk.Column, fk.ParentTable)
			issues = append(issues, migration.Issue{
				Rule:     RuleCode,
				Filename: fk.Filename,
				Line:     fk.Line,
				Message:  msg,
				Severity: "HIGH",
			})
		}
	}

	return issues
}
