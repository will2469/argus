package tests_test

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/will2469/argus/rules/a17_nplusone"
	"github.com/will2469/argus/rules/a24_tenant_leak"
	"github.com/will2469/argus/rules/a26_like_sanitize"
	"github.com/will2469/argus/shared/directives"
)

type MutationTestCase struct {
	Name           string
	Category       string
	Description    string
	ExpectedKilled bool // true means mutant MUST be detected and eliminated
	Execute        func(t *testing.T) (killed bool, reason string)
}

// TestMutation_ResilienceHarness verifies Layer 3 of the Argus Quality Pyramid:
// Evasion resilience and mutation testing against operator inversion, lexical spoofing,
// receiver masquerading, and pathological literals.
func TestMutation_ResilienceHarness(t *testing.T) {
	tcA24 := &a24_tenant_leak.TenantConfig{
		TenantColumn: "tenant_id",
		TenantTables: map[string]bool{"users": true, "orders": true},
	}

	mutations := []MutationTestCase{
		// Mutator A: Operator Inversion & Logic Tampering (ARGUS-A24)
		{
			Name:           "MutatorA1_ConjunctionToDisjunction",
			Category:       "Operator Inversion",
			Description:    "Flip AND to OR in tenant filter (WHERE tenant_id = $1 OR status = 'ACTIVE')",
			ExpectedKilled: true,
			Execute: func(t *testing.T) (bool, string) {
				sql := "SELECT id, name FROM users WHERE tenant_id = $1 OR status = 'ACTIVE'"
				violating, reason := a24_tenant_leak.CheckTenantQuery(sql, tcA24)
				return violating, reason
			},
		},
		{
			Name:           "MutatorA2_EqualityToInequality",
			Category:       "Operator Inversion",
			Description:    "Flip = to != in tenant predicate (WHERE tenant_id != $1 AND status = 'ACTIVE')",
			ExpectedKilled: true,
			Execute: func(t *testing.T) (bool, string) {
				sql := "SELECT id, name FROM users WHERE tenant_id != $1 AND status = 'ACTIVE'"
				violating, reason := a24_tenant_leak.CheckTenantQuery(sql, tcA24)
				return violating, reason
			},
		},
		{
			Name:           "MutatorA3_NullTestEvasion",
			Category:       "Operator Inversion",
			Description:    "Attempt tenant bypass with IS NOT NULL (WHERE tenant_id IS NOT NULL)",
			ExpectedKilled: true,
			Execute: func(t *testing.T) (bool, string) {
				sql := "SELECT id, name FROM users WHERE tenant_id IS NOT NULL"
				violating, reason := a24_tenant_leak.CheckTenantQuery(sql, tcA24)
				return violating, reason
			},
		},

		// Mutator B: Comment & Literal Spoofing (ARGUS-A24 & Directives)
		{
			Name:           "MutatorB1_SQLCommentSpoofing",
			Category:       "Lexical Spoofing",
			Description:    "Inject fake tenant clause in SQL comment (-- tenant_id = 1)",
			ExpectedKilled: true,
			Execute: func(t *testing.T) (bool, string) {
				sql := "SELECT id, email FROM users WHERE status = 'ACTIVE' -- tenant_id = $1"
				violating, reason := a24_tenant_leak.CheckTenantQuery(sql, tcA24)
				return violating, reason
			},
		},
		{
			Name:           "MutatorB2_StringLiteralSpoofing",
			Category:       "Lexical Spoofing",
			Description:    "Conceal tenant predicate in string constant (notes = 'tenant_id = 1')",
			ExpectedKilled: true,
			Execute: func(t *testing.T) (bool, string) {
				sql := "SELECT id, email FROM users WHERE notes = 'tenant_id = 1'"
				violating, reason := a24_tenant_leak.CheckTenantQuery(sql, tcA24)
				return violating, reason
			},
		},
		{
			Name:           "MutatorB3_SingleWordDirectiveRejection",
			Category:       "Directive Spoofing",
			Description:    "Reject single-word ignore comment without explanation (// argus:ignore ARGUS-A24 bypass)",
			ExpectedKilled: true,
			Execute: func(t *testing.T) (bool, string) {
				src := "package test\n// argus:ignore ARGUS-A24 bypass\nfunc run() {}\n"
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
				if err != nil {
					return false, err.Error()
				}
				dm := directives.ParseGoDirectives(file, fset)
				// Must NOT be suppressed because "bypass" is only 1 word (requires >= 2 words)
				ignored := dm.IsLineIgnored("test.go", 3, "ARGUS-A24")
				return !ignored, "1-word directive correctly rejected by directive gate"
			},
		},

		// Mutator C: Receiver & Method Identity Masquerading (ARGUS-A17 & A26)
		{
			Name:           "MutatorC1_FakeSanitizerMethodSpoofing",
			Category:       "Identity Masquerading",
			Description:    "Reject fake sanitizer on untrusted struct (evil.SanitizeLikePattern)",
			ExpectedKilled: true,
			Execute: func(t *testing.T) (bool, string) {
				src := `package test
import "context"
type Evil struct{}
func (Evil) SanitizeLikePattern(s string) string { return s }
func Search(ctx context.Context, evil Evil, input string) {
	pattern := evil.SanitizeLikePattern(input)
	db.Query(ctx, "SELECT * FROM users WHERE name LIKE $1", pattern)
}
`
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "test.go", src, 0)
				if err != nil {
					return false, err.Error()
				}
				dm := directives.NewDirectiveMap()
				issues := a26_like_sanitize.InspectFile(nil, fset, file, dm)
				return len(issues) > 0, "Fake sanitizer on untrusted receiver killed"
			},
		},
		{
			Name:           "MutatorC2_ReceiverCollisionInLoop",
			Category:       "Identity Masquerading",
			Description:    "Differentiate MemoryCache.Get (safe) vs DBRepo.Get (N+1 query)",
			ExpectedKilled: true,
			Execute: func(t *testing.T) (bool, string) {
				src := `package test
import "context"
type MemoryCache struct{}
func (MemoryCache) Get(id int) string { return "cached" }
type DBRepo struct{}
func (DBRepo) Get(ctx context.Context, id int) { db.Query(ctx, "SELECT id FROM users WHERE id = $1", id) }
func CacheLoop(cache MemoryCache, ids []int) {
	for _, id := range ids { cache.Get(id) }
}
func DBLoop(ctx context.Context, repo DBRepo, ids []int) {
	for _, id := range ids { repo.Get(ctx, id) }
}
`
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "test.go", src, 0)
				if err != nil {
					return false, err.Error()
				}
				dm := directives.NewDirectiveMap()
				detector := a17_nplusone.NewHelperQueryDetector(nil, file)
				issues := a17_nplusone.WalkLoops(nil, fset, file, dm, detector)
				// Must detect DBLoop, but NOT CacheLoop
				killed := len(issues) == 1
				return killed, "A17 correctly disambiguated DBRepo.Get vs MemoryCache.Get"
			},
		},

		// Mutator D: Pathological Literals & Fail-Closed Invariant (ARGUS-A26 & A24)
		{
			Name:           "MutatorD1_PureWildcardLiteralDoS",
			Category:       "Pathological Literals",
			Description:    "Detect standalone pure wildcard '%' as CWE-400 table scan DoS",
			ExpectedKilled: true,
			Execute: func(t *testing.T) (bool, string) {
				src := `package test
import "context"

func WildcardQuery(ctx context.Context) {
	db.Query(ctx, "SELECT id FROM orders WHERE status LIKE $1", "%")
}
`
				fset := token.NewFileSet()
				file, err := parser.ParseFile(fset, "test.go", src, 0)
				if err != nil {
					return false, err.Error()
				}
				dm := directives.NewDirectiveMap()
				issues := a26_like_sanitize.InspectFile(nil, fset, file, dm)
				return len(issues) > 0, "Pure wildcard literal DoS killed"
			},
		},
		{
			Name:           "MutatorD2_FailClosedUnparseableQueryOnTenantTable",
			Category:       "Fail-Closed Invariant",
			Description:    "Reject unparseable query referencing multi-tenant table without falling back to weak heuristics",
			ExpectedKilled: true,
			Execute: func(t *testing.T) (bool, string) {
				unparseableSQL := "SELECT * FROM users WHERE [unparseable broken syntax ???]"
				violating, reason := a24_tenant_leak.CheckTenantQuery(unparseableSQL, tcA24)
				hasParseErr := strings.Contains(reason, "SQL AST parsing failed")
				return violating && hasParseErr, reason
			},
		},
	}

	t.Log("=========================================================================================================")
	t.Log("ARGUS 1-SSOT GOLDEN CORPUS — GATE 3: MUTATION & EVASION RESILIENCE HARNESS")
	t.Log("=========================================================================================================")
	t.Logf("%-32s | %-24s | %-12s | %s", "MUTANT IDENTIFIER", "CATEGORY", "STATUS", "REASON")
	t.Log("---------------------------------------------------------------------------------------------------------")

	killedCount := 0
	for _, tc := range mutations {
		killed, reason := tc.Execute(t)
		status := "SURVIVED (FAIL)"
		if killed == tc.ExpectedKilled {
			status = "KILLED (PASS)"
			killedCount++
		} else {
			t.Errorf("MUTATION SURVIVAL: Mutant %s was expected to be killed, but survived! Reason: %s", tc.Name, reason)
		}
		t.Logf("%-32s | %-24s | %-12s | %s", tc.Name, tc.Category, status, reason)
	}

	t.Log("=========================================================================================================")
	killRate := float64(killedCount) / float64(len(mutations)) * 100
	t.Logf("Mutation Testing Summary:")
	t.Logf("  Total Mutants Evaluated : %d", len(mutations))
	t.Logf("  Total Mutants Killed    : %d", killedCount)
	t.Logf("  Surviving Mutants       : %d", len(mutations)-killedCount)
	t.Logf("  Mutation Kill Rate      : %.1f%% (Target: 100.0%%)", killRate)
	t.Log("=========================================================================================================")

	if killedCount != len(mutations) {
		t.Fatalf("GATE 3 FAILED: Mutation kill rate is %.1f%% (must be 100.0%%)", killRate)
	}
}
