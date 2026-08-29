package runner

import (
	"regexp"
	"sync"
	"time"
)

var paramRegex = regexp.MustCompile(`\$\d+`)

// Issue represents a single database query violation found during standalone scan.
type Issue struct {
	File     string
	Line     int
	Rule     string
	Message  string
	Snippet  string
	Category string
}

// RuleAuditInfo captures the dynamic execution state of a specific rule.
type RuleAuditInfo struct {
	ID                string
	Code              string
	Description       string
	CheckedComponents int
	IssuesFound       int
	Status            string // "PASS" or "FAILED"
}

// AuditResult aggregates metrics and issues for reporting.
type AuditResult struct {
	Timestamp                  time.Time
	ScannedFiles               int
	ScannedGoFiles             int
	ScannedMigrationFiles      int
	VerifiedQuerySites         int
	VerifiedParameterizedSites int
	AttachedRules              []RuleAuditInfo
	Issues                     []Issue
	Score                      float64
	Grade                      string
}

// MetricsTracker aggregates scanning counters in a thread-safe manner.
type MetricsTracker struct {
	mu                         sync.Mutex
	scannedFiles               int
	scannedGoFiles             int
	scannedMigrationFiles      int
	verifiedQuerySites         int
	verifiedParameterizedSites int
	attachedRules              []RuleAuditInfo
	issues                     []Issue
}

// NewMetricsTracker creates an initialized MetricsTracker.
func NewMetricsTracker() *MetricsTracker {
	return &MetricsTracker{}
}

// CountParameters returns the count of $1, $2, ... placeholders in SQL query.
func CountParameters(sql string) int {
	return len(paramRegex.FindAllString(sql, -1))
}

func (m *MetricsTracker) IncrementScannedFiles(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scannedFiles += n
	m.scannedGoFiles += n
}

func (m *MetricsTracker) IncrementMigrationFiles(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scannedFiles += n
	m.scannedMigrationFiles += n
}

func (m *MetricsTracker) IncrementQuerySites(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifiedQuerySites += n
}

func (m *MetricsTracker) IncrementParameterizedSites(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifiedParameterizedSites += n
}

func (m *MetricsTracker) SetAttachedRules(rules []RuleAuditInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attachedRules = rules
}

func (m *MetricsTracker) AddIssue(issue Issue) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.issues = append(m.issues, issue)
}

func (m *MetricsTracker) Snapshot() *AuditResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	score, grade := calculateScoreAndGrade(m.issues)

	rulesCopy := make([]RuleAuditInfo, len(m.attachedRules))
	copy(rulesCopy, m.attachedRules)

	issuesCopy := make([]Issue, len(m.issues))
	copy(issuesCopy, m.issues)

	return &AuditResult{
		Timestamp:                  time.Now().UTC(),
		ScannedFiles:               m.scannedFiles,
		ScannedGoFiles:             m.scannedGoFiles,
		ScannedMigrationFiles:      m.scannedMigrationFiles,
		VerifiedQuerySites:         m.verifiedQuerySites,
		VerifiedParameterizedSites: m.verifiedParameterizedSites,
		AttachedRules:              rulesCopy,
		Issues:                     issuesCopy,
		Score:                      score,
		Grade:                      grade,
	}
}

func calculateScoreAndGrade(issues []Issue) (float64, string) {
	if len(issues) == 0 {
		return 100.0, "A+"
	}

	penalty := 0.0
	for _, issue := range issues {
		switch issue.Category {
		case "security":
			penalty += 15.0
		case "reliability":
			penalty += 10.0
		case "performance":
			penalty += 5.0
		default:
			penalty += 5.0
		}
	}

	score := 100.0 - penalty
	if score < 0.0 {
		score = 0.0
	}

	grade := "A+"
	switch {
	case score >= 95.0:
		grade = "A+"
	case score >= 90.0:
		grade = "A"
	case score >= 80.0:
		grade = "B"
	case score >= 70.0:
		grade = "C"
	case score >= 60.0:
		grade = "D"
	default:
		grade = "F"
	}

	return score, grade
}
