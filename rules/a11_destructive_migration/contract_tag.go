// Package a11_destructive_migration provides validation for contract-phase annotations and evidence.
package a11_destructive_migration

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/will2469/argus/shared/directives"
)

// MigrationPhaseMetadata captures phase declaration and tracking evidence for a migration.
type MigrationPhaseMetadata struct {
	Phase      string // "contract", "expand", "migrate"
	Release    string // e.g. "v2.0.0", "release_v2"
	Issue      string // e.g. "PROJ-1234", "#456"
	ApprovedBy string // e.g. "alice", "dba-team"
	Reason     string // descriptive reason
	Line       int    // line where metadata was declared
	Raw        string // raw comment string
}

// ContractValidationResult reports whether contract evidence is valid or explains why it failed.
type ContractValidationResult struct {
	IsValid  bool
	Reason   string
	Metadata MigrationPhaseMetadata
}

var (
	dummyReleaseBlacklist = map[string]bool{
		"anything": true,
		"dummy":    true,
		"test":     true,
		"foo":      true,
		"bar":      true,
		"true":     true,
		"false":    true,
		"1":        true,
		"temp":     true,
		"skip":     true,
		"ignore":   true,
		"none":     true,
		"all":      true,
		"whatever": true,
	}

	reSemVerOrRelease = regexp.MustCompile(`(?i)^(?:v?[0-9]+(?:\.[0-9]+)*.*|release[-_][a-zA-Z0-9_.-]+|rel[-_][a-zA-Z0-9_.-]+|[a-zA-Z0-9_.-]+-v?[0-9]+.*)$`)
	reKeyValue        = regexp.MustCompile(`([a-zA-Z0-9_-]+)=("([^"]*)"|'([^']*)'|([^\s]+))`)
)

// ExtractMigrationPhaseMetadata extracts contract phase metadata from preceding comments, file header, or path.
func ExtractMigrationPhaseMetadata(filename, content string, stmtLine int) *MigrationPhaseMetadata {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	// 1. Check preceding comments (up to 5 lines above stmtLine)
	if stmtLine > 1 {
		start := stmtLine - 5
		if start < 1 {
			start = 1
		}
		for i := stmtLine - 2; i >= start-1; i-- {
			if i >= len(lines) {
				continue
			}
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" {
				continue
			}
			if !strings.HasPrefix(trimmed, "--") {
				break
			}
			if meta := parseContractComment(trimmed, i+1); meta != nil {
				return meta
			}
		}
	}

	// 2. Check file-level header comments (first 10 lines)
	limit := 10
	if len(lines) < limit {
		limit = len(lines)
	}
	for i := 0; i < limit; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "--") {
			if trimmed != "" {
				break
			}
			continue
		}
		if meta := parseContractComment(trimmed, i+1); meta != nil {
			return meta
		}
	}

	// 3. Path / Directory convention
	cleanPath := filepath.ToSlash(strings.ToLower(filename))
	if strings.Contains(cleanPath, "/contract/") || strings.Contains(cleanPath, "/contract_phase/") || strings.HasSuffix(cleanPath, "_contract.up.sql") {
		return &MigrationPhaseMetadata{
			Phase: "contract",
		}
	}

	return nil
}

func parseContractComment(line string, lineNum int) *MigrationPhaseMetadata {
	comment := strings.TrimSpace(strings.TrimPrefix(line, "--"))
	lower := strings.ToLower(comment)

	if !strings.HasPrefix(lower, "argus:contract") && !strings.HasPrefix(lower, "argus:phase contract") {
		return nil
	}

	meta := &MigrationPhaseMetadata{
		Phase: "contract",
		Line:  lineNum,
		Raw:   comment,
	}

	// Remove directive prefix
	var payload string
	if strings.HasPrefix(lower, "argus:contract") {
		payload = strings.TrimSpace(comment[len("argus:contract"):])
	} else if strings.HasPrefix(lower, "argus:phase contract") {
		payload = strings.TrimSpace(comment[len("argus:phase contract"):])
	}

	if payload == "" {
		return meta
	}

	// Parse key=value pairs if present
	matches := reKeyValue.FindAllStringSubmatch(payload, -1)
	if len(matches) > 0 {
		for _, m := range matches {
			k := strings.ToLower(m[1])
			v := m[3]
			if v == "" {
				v = m[4]
			}
			if v == "" {
				v = m[5]
			}
			switch k {
			case "phase":
				meta.Phase = strings.ToLower(v)
			case "release", "ver", "version":
				meta.Release = v
			case "issue", "ticket", "task":
				meta.Issue = v
			case "approved_by", "signoff", "approver":
				meta.ApprovedBy = v
			case "reason", "rationale", "justification":
				meta.Reason = v
			}
		}
		return meta
	}

	// Positional format: tokens
	tokens := strings.Fields(payload)
	if len(tokens) > 0 {
		meta.Release = tokens[0]
		if len(tokens) > 1 {
			meta.Reason = strings.Join(tokens[1:], " ")
		}
	}

	return meta
}

// ValidateContractEvidence verifies that contract metadata represents valid, accountable contract evidence.
func ValidateContractEvidence(meta *MigrationPhaseMetadata) ContractValidationResult {
	if meta == nil || meta.Phase != "contract" {
		return ContractValidationResult{
			IsValid: false,
			Reason:  "no contract phase declared",
		}
	}

	if meta.Release == "" {
		return ContractValidationResult{
			IsValid: false,
			Reason:  "missing required release identifier in contract metadata",
		}
	}

	lowerRelease := strings.ToLower(meta.Release)
	if dummyReleaseBlacklist[lowerRelease] {
		return ContractValidationResult{
			IsValid: false,
			Reason:  fmt.Sprintf("dummy release tag %q is not valid contract evidence", meta.Release),
		}
	}

	if !reSemVerOrRelease.MatchString(meta.Release) {
		return ContractValidationResult{
			IsValid: false,
			Reason:  fmt.Sprintf("release identifier %q lacks version or release structure", meta.Release),
		}
	}

	// Validate accountability evidence: requires issue, approver, multi-word reason, or qualified release task
	hasIssue := len(meta.Issue) > 0
	hasApprover := len(meta.ApprovedBy) > 0
	hasMultiWordReason := len(strings.Fields(meta.Reason)) >= 2
	isQualifiedReleaseTask := strings.Contains(lowerRelease, "cleanup") ||
		strings.Contains(lowerRelease, "drop") ||
		strings.Contains(lowerRelease, "contract") ||
		strings.Contains(lowerRelease, "deprecat")

	if !hasIssue && !hasApprover && !hasMultiWordReason && !isQualifiedReleaseTask {
		return ContractValidationResult{
			IsValid: false,
			Reason:  "contract evidence missing accountability artifact (issue ticket, approver, or multi-word deprecation reason)",
		}
	}

	return ContractValidationResult{
		IsValid:  true,
		Metadata: *meta,
	}
}

// IsContractTagged provides compatibility check for contract evidence or directive suppression.
func IsContractTagged(filename, content string, line int, dm *directives.DirectiveMap) bool {
	if dm != nil && dm.IsLineIgnored(filename, line, RuleCode) {
		return true
	}

	meta := ExtractMigrationPhaseMetadata(filename, content, line)
	res := ValidateContractEvidence(meta)
	return res.IsValid
}
