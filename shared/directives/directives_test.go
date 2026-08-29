package directives

import (
	"testing"
)

func TestDirectiveMap_CanonicalShortRule(t *testing.T) {
	dm := NewDirectiveMap()
	// // argus:ignore-a07 <alasan>
	dm.AddDirective("service.go", 10, "a07", "valid diagnostic reason")

	// Matches ARGUS-A07
	if !dm.IsLineIgnored("service.go", 10, "ARGUS-A07") {
		t.Errorf("expected line 10 to be ignored for ARGUS-A07 via a07 shortcode")
	}

	// Line right after directive (line 11)
	if !dm.IsLineIgnored("service.go", 11, "ARGUS-A07") {
		t.Errorf("expected line 11 to be ignored for ARGUS-A07")
	}

	// Line 2 lines after directive (line 12, multi-line statement formatting)
	if !dm.IsLineIgnored("service.go", 12, "ARGUS-A07") {
		t.Errorf("expected line 12 to be ignored for multi-line span")
	}

	// Different rule should not be ignored
	if dm.IsLineIgnored("service.go", 11, "ARGUS-A08") {
		t.Errorf("expected ARGUS-A08 not to be ignored")
	}
}

func TestDirectiveMap_ClauseDotNotation(t *testing.T) {
	dm := NewDirectiveMap()
	// // argus:ignore-a07.detail <alasan>
	dm.AddDirective("handler.go", 20, "a07.detail", "diagnostic log output")

	// Matches base rule ARGUS-A07
	if !dm.IsLineIgnored("handler.go", 20, "ARGUS-A07") {
		t.Errorf("expected base ARGUS-A07 to match directive a07.detail")
	}

	// Matches specific clause
	if !dm.IsLineIgnored("handler.go", 20, "ARGUS-A07.DETAIL") {
		t.Errorf("expected clause ARGUS-A07.DETAIL to match")
	}
}

func TestDirectiveMap_ShortReasonRejected(t *testing.T) {
	dm := NewDirectiveMap()
	// Only 1 word -> should be rejected!
	dm.AddDirective("test.go", 10, "a07", "bypass")

	if dm.IsLineIgnored("test.go", 10, "ARGUS-A07") {
		t.Errorf("expected 1-word reason to be rejected and not ignored")
	}
}

func TestParseSQLDirectives_CanonicalHyphen(t *testing.T) {
	sql := `
-- Safe table creation
CREATE TABLE users (id int);

-- argus:ignore-a29.foreign-key legacy dictionary table kept
ALTER TABLE orders ADD CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id);
`
	dm := ParseSQLDirectives(sql, "001_orders.sql")
	if !dm.IsLineIgnored("001_orders.sql", 6, "ARGUS-A29") {
		t.Errorf("expected line 6 in SQL migration to be ignored via -- argus:ignore-a29.foreign-key")
	}
}

func TestDirectiveMap_DynamicRulePattern(t *testing.T) {
	dm := NewDirectiveMap()
	// Unregistered future rules (e.g. A18, A99, A150) should work out of the box
	dm.AddDirective("future.go", 10, "a18", "future rows error check")
	dm.AddDirective("future.go", 20, "A99", "future ninety nine rule")
	dm.AddDirective("future.go", 30, "argus-a150", "future large number rule")

	if !dm.IsLineIgnored("future.go", 10, "ARGUS-A18") {
		t.Errorf("expected a18 to normalize to ARGUS-A18")
	}
	if !dm.IsLineIgnored("future.go", 20, "ARGUS-A99") {
		t.Errorf("expected A99 to normalize to ARGUS-A99")
	}
	if !dm.IsLineIgnored("future.go", 30, "ARGUS-A150") {
		t.Errorf("expected argus-a150 to normalize to ARGUS-A150")
	}
}

func TestDirectiveMap_AliasCaseAndSeparatorAgnostic(t *testing.T) {
	dm := NewDirectiveMap()
	dm.AddDirective("repo.go", 15, "FORBIDDEN_QUERY_IN_LOOP", "valid batch loop")

	// Matches canonical code
	if !dm.IsLineIgnored("repo.go", 15, "ARGUS-A17") {
		t.Errorf("expected FORBIDDEN_QUERY_IN_LOOP to map to ARGUS-A17")
	}

	// Hyphenated alias query
	if !dm.IsLineIgnored("repo.go", 15, "FORBIDDEN-QUERY-IN-LOOP") {
		t.Errorf("expected FORBIDDEN-QUERY-IN-LOOP to match")
	}
}

func TestDirectiveMap_RegisterAlias(t *testing.T) {
	dm := NewDirectiveMap()
	RegisterAlias("CUSTOM_NEW_RULE_ALIAS", "ARGUS-A50")
	dm.AddDirective("custom.go", 5, "custom_new_rule_alias", "valid custom reason")

	if !dm.IsLineIgnored("custom.go", 5, "ARGUS-A50") {
		t.Errorf("expected dynamically registered alias to resolve to ARGUS-A50")
	}
}

func TestDirectiveMap_WildcardAll(t *testing.T) {
	dm := NewDirectiveMap()
	dm.AddDirective("vendor.go", 1, "ALL", "third party legacy code")

	if !dm.IsLineIgnored("vendor.go", 1, "ARGUS-A01") {
		t.Errorf("expected ALL to suppress ARGUS-A01")
	}
	if !dm.IsLineIgnored("vendor.go", 1, "ARGUS-A17") {
		t.Errorf("expected ALL to suppress ARGUS-A17")
	}
}
