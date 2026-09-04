package a07_error_leak_test

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/will2469/argus/rules/a07_error_leak"
)

func parseAndInspectA07(t *testing.T, src string) ([]a07_error_leak.Issue, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	issues := a07_error_leak.InspectFile(nil, fset, file, nil)
	return issues, fset
}

// 1. Branching Conditional DB Error — lattice join ensures MAYBE_DB is flagged.
func TestLattice_BranchingConditionalDBError(t *testing.T) {
	src := `package test
import (
	"database/sql"
	"net/http"
)
func handler(w http.ResponseWriter, db *sql.DB, cond bool) {
	var err error
	if cond {
		_, err = db.Query("SELECT 1")
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}
`
	issues, fset := parseAndInspectA07(t, src)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue for conditional DB error join, got %d", len(issues))
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 12 {
		t.Errorf("expected issue on line 12, got line %d", pos.Line)
	}
}

// 2. Sequential Clean Override — DB error reassigned to clean errors.New must NOT be flagged.
func TestLattice_SequentialCleanOverride(t *testing.T) {
	src := `package test
import (
	"database/sql"
	"errors"
	"net/http"
)
func handler(w http.ResponseWriter, db *sql.DB) {
	_, err := db.Query("SELECT 1")
	err = errors.New("sanitized message")
	http.Error(w, err.Error(), 500)
}
`
	issues, _ := parseAndInspectA07(t, src)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for clean sequential override, got %d", len(issues))
	}
}

// 3. Sequential Dirty Override — clean error reassigned to DB query must be flagged.
func TestLattice_SequentialDirtyOverride(t *testing.T) {
	src := `package test
import (
	"database/sql"
	"errors"
	"net/http"
)
func handler(w http.ResponseWriter, db *sql.DB) {
	err := errors.New("initial message")
	_, err = db.Query("SELECT 1")
	http.Error(w, err.Error(), 500)
}
`
	issues, fset := parseAndInspectA07(t, src)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue for dirty sequential override, got %d", len(issues))
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 10 {
		t.Errorf("expected issue on line 10, got line %d", pos.Line)
	}
}

// 4. Variable Shadowing — inner scope shadows error with errors.New, outer scope leaks DB error.
func TestLattice_VariableShadowing(t *testing.T) {
	src := `package test
import (
	"database/sql"
	"errors"
	"net/http"
)
func handler(w http.ResponseWriter, db *sql.DB) {
	_, err := db.Query("SELECT 1")
	{
		err := errors.New("inner safe error")
		http.Error(w, err.Error(), 400) // Line 11: inner clean must NOT be flagged
	}
	http.Error(w, err.Error(), 500) // Line 13: outer DB error MUST be flagged
}
`
	issues, fset := parseAndInspectA07(t, src)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue for outer shadowed DB error, got %d", len(issues))
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 13 {
		t.Errorf("expected issue on line 13, got line %d", pos.Line)
	}
}

// 5. Calculator Non-DB — receiver named db with type Calculator must NOT be flagged.
func TestLattice_CalculatorNonDB(t *testing.T) {
	src := `package test
import (
	"net/http"
)
type Calculator struct{}
func (Calculator) Exec(string) error { return nil }

func handler(w http.ResponseWriter) {
	var db Calculator
	err := db.Exec("hello")
	http.Error(w, err.Error(), 500)
}
`
	issues, _ := parseAndInspectA07(t, src)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for non-DB Calculator receiver, got %d", len(issues))
	}
}

// 6. SqlErr Named Non-DB — variable named sqlErr created via errors.New must NOT be flagged.
func TestLattice_SqlErrNamedNonDB(t *testing.T) {
	src := `package test
import (
	"errors"
	"net/http"
)
func handler(w http.ResponseWriter) {
	sqlErr := errors.New("not database related")
	http.Error(w, sqlErr.Error(), 400)
}
`
	issues, _ := parseAndInspectA07(t, src)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for sqlErr name on non-DB error, got %d", len(issues))
	}
}

// 7. ValErr Named DB — variable named valErr originating from DB query MUST be flagged.
func TestLattice_ValErrNamedDB(t *testing.T) {
	src := `package test
import (
	"database/sql"
	"net/http"
)
func handler(w http.ResponseWriter, db *sql.DB) {
	_, valErr := db.Query("SELECT 1")
	http.Error(w, valErr.Error(), 500)
}
`
	issues, fset := parseAndInspectA07(t, src)
	if len(issues) != 1 {
		t.Fatalf("expected exactly 1 issue for valErr assigned from DB call, got %d", len(issues))
	}
	pos := fset.Position(issues[0].Pos)
	if pos.Line != 8 {
		t.Errorf("expected issue on line 8, got line %d", pos.Line)
	}
}
