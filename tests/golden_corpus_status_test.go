package tests_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type RuleCorpusStatus struct {
	RuleCode  string
	Category  string
	IsAdopted bool
	Path      string
	Details   string
}

// CheckRuleGoldenStatus inspects the repository to evaluate if a given rule has adopted
// the 1 SSOT Golden Adversarial Corpus standard.
func CheckRuleGoldenStatus(rootDir string, ruleNum int) RuleCorpusStatus {
	id := fmt.Sprintf("A%02d", ruleNum)
	code := fmt.Sprintf("ARGUS-%s", id)
	folder := strings.ToLower(id)

	// Determine category (migration vs correctness)
	isMigration := false
	switch id {
	case "A11", "A13", "A15", "A27", "A28", "A29", "A30":
		isMigration = true
	}

	if isMigration {
		migPath := filepath.Join(rootDir, "tests", "migration", folder)
		if info, err := os.Stat(migPath); err == nil && info.IsDir() {
			return RuleCorpusStatus{
				RuleCode:  code,
				Category:  "Migration",
				IsAdopted: true,
				Path:      filepath.Join("tests", "migration", folder),
				Details:   "Golden migration corpus verified",
			}
		}
		return RuleCorpusStatus{
			RuleCode:  code,
			Category:  "Migration",
			IsAdopted: false,
			Path:      filepath.Join("testdata", "src", folder),
			Details:   "Pending migration to tests/migration/" + folder,
		}
	}

	// Correctness rule
	corrPath := filepath.Join(rootDir, "tests", "correctness", folder)
	posFile := filepath.Join(corrPath, "positive", "positive.go")
	negFile := filepath.Join(corrPath, "negative", "negative.go")
	advFile := filepath.Join(corrPath, "adversarial", "adversarial.go")
	testFile := filepath.Join(corrPath, fmt.Sprintf("%s_corpus_test.go", folder))

	hasPos := fileExists(posFile)
	hasNeg := fileExists(negFile)
	hasAdv := fileExists(advFile)
	hasTest := fileExists(testFile)

	if hasPos && hasNeg && hasAdv && hasTest {
		return RuleCorpusStatus{
			RuleCode:  code,
			Category:  "Correctness",
			IsAdopted: true,
			Path:      filepath.Join("tests", "correctness", folder),
			Details:   "1 SSOT Golden Corpus (P1-P5, N1-N5, A1-A7, analysistest + runner)",
		}
	}

	return RuleCorpusStatus{
		RuleCode:  code,
		Category:  "Correctness",
		IsAdopted: false,
		Path:      filepath.Join("testdata", "src", folder),
		Details:   "Legacy unit-level testdata fixture (pending adoption)",
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func TestGoldenCorpus_AdoptionMatrix(t *testing.T) {
	rootDir, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("failed to resolve root dir: %v", err)
	}

	adoptedCount := 0
	totalRules := 30

	t.Log("=========================================================================================================")
	t.Logf("%-12s | %-12s | %-10s | %-32s | %s", "RULE CODE", "CATEGORY", "STATUS", "SSOT PATH", "DETAILS")
	t.Log("---------------------------------------------------------------------------------------------------------")

	for i := 1; i <= totalRules; i++ {
		status := CheckRuleGoldenStatus(rootDir, i)
		statusStr := "PENDING"
		if status.IsAdopted {
			statusStr = "ADOPTED"
			adoptedCount++
		}

		t.Logf("%-12s | %-12s | %-10s | %-32s | %s",
			status.RuleCode,
			status.Category,
			statusStr,
			status.Path,
			status.Details,
		)
	}
	t.Log("=========================================================================================================")
	t.Logf("Golden Corpus Adoption Progress: %d / %d rules adopted (%.1f%%)",
		adoptedCount, totalRules, float64(adoptedCount)/float64(totalRules)*100)

	// Assert that A01 is verified as adopted
	a01Status := CheckRuleGoldenStatus(rootDir, 1)
	if !a01Status.IsAdopted {
		t.Errorf("expected ARGUS-A01 to be ADOPTED as 1 SSOT Golden Corpus, but got: %+v", a01Status)
	}
}
