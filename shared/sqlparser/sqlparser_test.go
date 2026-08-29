package sqlparser

import (
	"testing"
)

func TestParse_Basic(t *testing.T) {
	tree, err := Parse("SELECT id, name FROM users WHERE id = $1")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(tree.Stmts) == 0 {
		t.Fatal("expected at least 1 statement")
	}

	selectStmt := tree.Stmts[0].Stmt.GetSelectStmt()
	if selectStmt == nil {
		t.Fatal("expected SelectStmt")
	}

	tables := ExtractFromTableNames(selectStmt)
	if len(tables) != 1 || tables[0] != "users" {
		t.Errorf("expected table 'users', got %v", tables)
	}

	if HasSelectStar(selectStmt) {
		t.Errorf("expected HasSelectStar to be false")
	}
}

func TestHasSelectStar(t *testing.T) {
	tree, err := Parse("SELECT * FROM users")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt := tree.Stmts[0].Stmt.GetSelectStmt()
	if !HasSelectStar(selectStmt) {
		t.Errorf("expected HasSelectStar to be true for SELECT *")
	}

	// Aggregate COUNT(*) should NOT trigger HasSelectStar
	treeCount, err := Parse("SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectCount := treeCount.Stmts[0].Stmt.GetSelectStmt()
	if HasSelectStar(selectCount) {
		t.Errorf("expected COUNT(*) NOT to trigger HasSelectStar")
	}
}

func TestExtractLockingInfo(t *testing.T) {
	tree, err := Parse("SELECT id FROM task_queue WHERE status = 'PENDING' LIMIT 1 FOR UPDATE SKIP LOCKED")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	selectStmt := tree.Stmts[0].Stmt.GetSelectStmt()
	hasLock, isForUpdate, isSkipLocked, isNoWait := ExtractLockingInfo(selectStmt)

	if !hasLock || !isForUpdate || !isSkipLocked || isNoWait {
		t.Errorf("expected FOR UPDATE SKIP LOCKED (hasLock=true, isForUpdate=true, isSkipLocked=true, isNoWait=false), got (%v, %v, %v, %v)",
			hasLock, isForUpdate, isSkipLocked, isNoWait)
	}
}

func TestCollectCreatedTables(t *testing.T) {
	sql := `
CREATE TABLE users (id UUID PRIMARY KEY);
CREATE TABLE IF NOT EXISTS orders (id UUID PRIMARY KEY);
`
	tree, err := Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	created := CollectCreatedTables(tree)
	if !created["users"] || !created["orders"] {
		t.Errorf("expected 'users' and 'orders' in created tables, got %v", created)
	}
	if created["nonexistent"] {
		t.Errorf("did not expect 'nonexistent' in created tables")
	}
}
