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
	Reason     string // descriptive reason (minimum 2 words)
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
		"???": true, "?": true, "anything": true, "dummy": true, "test": true,
		"foo": true, "bar": true, "true": true, "false": true, "1": true,
		"temp": true, "skip": true, "ignore": true, "none": true, "all": true,
		"whatever": true, "todo": true, "tbd": true, "na": true, "n/a": true,
		"unknown": true, "null": true,
	}

	reSemVerOrRelease = regexp.MustCompile(`(?i)^(?:v?[0-9]+(?:\.[0-9]+)*.*|release[-_][a-zA-Z0-9_.-]+|rel[-_][a-zA-Z0-9_.-]+|[a-zA-Z0-9_.-]+-v?[0-9]+.*)$`)
	reKeyValue        = regexp.MustCompile(`([a-zA-Z0-9_-]+)[=:](?:["']([^"']*)["']|([^\s]+))`)
)

// ExtractMigrationPhaseMetadata extracts contract phase metadata from preceding comments, file header, or path.
func ExtractMigrationPhaseMetadata(filename, content string, stmtLine int) *MigrationPhaseMetadata {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	// 1. Check contiguous preceding comments (up to 10 lines above stmtLine)
	if stmtLine > 1 {
		var commentLines []string
		commentStartLine := stmtLine
		for i := stmtLine - 2; i >= 0 && i >= stmtLine-12; i-- {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" {
				continue
			}
			if !strings.HasPrefix(trimmed, "--") {
				break
			}
			commentLines = append([]string{trimmed}, commentLines...)
			commentStartLine = i + 1
		}
		if meta := parseCommentBlock(commentLines, commentStartLine); meta != nil {
			return meta
		}
	}

	// 2. Check file-level header comments (first 10 lines)
	limit := 10
	if len(lines) < limit {
		limit = len(lines)
	}
	var headerComments []string
	for i := 0; i < limit; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "--") {
			if trimmed != "" {
				break
			}
			continue
		}
		headerComments = append(headerComments, trimmed)
	}
	if meta := parseCommentBlock(headerComments, 1); meta != nil {
		return meta
	}

	// 3. Path / Directory convention
	cleanPath := filepath.ToSlash(strings.ToLower(filename))
	if strings.Contains(cleanPath, "/contract/") || strings.Contains(cleanPath, "/contract_phase/") || strings.HasSuffix(cleanPath, "_contract.up.sql") {
		return &MigrationPhaseMetadata{Phase: "contract"}
	}
	return nil
}

func parseCommentBlock(lines []string, startLine int) *MigrationPhaseMetadata {
	var meta *MigrationPhaseMetadata
	for idx, line := range lines {
		raw := strings.TrimSpace(strings.TrimPrefix(line, "--"))
		if !strings.HasPrefix(strings.ToLower(raw), "argus:") {
			continue
		}
		if meta == nil {
			meta = &MigrationPhaseMetadata{Phase: "contract", Line: startLine + idx, Raw: raw}
		}
		parseLineIntoMetadata(raw, meta)
	}
	return meta
}

func parseLineIntoMetadata(raw string, meta *MigrationPhaseMetadata) {
	body := strings.TrimSpace(raw[len("argus:"):])
	if body == "" {
		return
	}

	// 1. Try key=value or key: value pairs
	matches := reKeyValue.FindAllStringSubmatch(body, -1)
	if len(matches) > 0 {
		for _, m := range matches {
			k := strings.ToLower(m[1])
			v := m[2]
			if v == "" {
				v = m[3]
			}
			if k == "contract" {
				meta.Release = v
				meta.Phase = "contract"
			} else {
				assignMetaField(meta, k, v)
			}
		}
		return
	}

	// 2. Positional format
	tokens := strings.Fields(body)
	if len(tokens) == 0 {
		return
	}

	if tokens[0] == "phase" {
		if len(tokens) > 1 {
			meta.Phase = strings.ToLower(tokens[1])
			tokens = tokens[2:]
		} else {
			tokens = tokens[1:]
		}
	} else if tokens[0] == "contract" {
		tokens = tokens[1:]
	}

	if len(tokens) > 0 {
		meta.Release = tokens[0]
		if len(tokens) > 1 {
			meta.Reason = strings.Join(tokens[1:], " ")
		}
	}
}

func assignMetaField(meta *MigrationPhaseMetadata, k, v string) {
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

// ValidateContractEvidence verifies that contract metadata represents authoritative contract evidence.
func ValidateContractEvidence(meta *MigrationPhaseMetadata) ContractValidationResult {
	if meta == nil || meta.Phase == "" {
		return ContractValidationResult{IsValid: false, Reason: "no contract phase declared"}
	}
	if meta.Phase != "contract" {
		return ContractValidationResult{
			IsValid: false,
			Reason:  fmt.Sprintf("operation only permitted in contract phase (current phase: %q)", meta.Phase),
		}
	}
	if meta.Release == "" {
		return ContractValidationResult{IsValid: false, Reason: "missing required release identifier in contract metadata"}
	}

	lowerRelease := strings.ToLower(strings.TrimSpace(meta.Release))
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
	return ContractValidationResult{IsValid: true, Metadata: *meta}
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
