// Package a10_isolation_level detects pessimistic row locking and table-correlated advisory lock protection.
package a10_isolation_level

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/callsite"
	"github.com/will2469/argus/shared/sqlparser"
)

// ExtractLockedTables extracts the list of database tables locked via SELECT ... FOR UPDATE or FOR NO KEY UPDATE.
func ExtractLockedTables(sql string) []TableRef {
	tree, err := sqlparser.Parse(sql)
	if err != nil {
		return heuristicExtractLockedTables(sql)
	}

	var locked []TableRef
	for _, stmt := range tree.Stmts {
		if stmt.Stmt == nil {
			continue
		}
		if sel := stmt.Stmt.GetSelectStmt(); sel != nil {
			if isPessimisticLock(sel.LockingClause) {
				extractTablesFromClause(sel.FromClause, &locked)
			}
		}
	}
	return locked
}

func isPessimisticLock(clauses []*pg_query.Node) bool {
	for _, lc := range clauses {
		if clause := lc.GetLockingClause(); clause != nil {
			if clause.Strength == pg_query.LockClauseStrength_LCS_FORUPDATE ||
				clause.Strength == pg_query.LockClauseStrength_LCS_FORNOKEYUPDATE {
				return true
			}
		}
	}
	return false
}

func extractTablesFromClause(fromList []*pg_query.Node, tables *[]TableRef) {
	for _, node := range fromList {
		if node == nil {
			continue
		}
		if rv := node.GetRangeVar(); rv != nil && rv.Relname != "" {
			*tables = append(*tables, TableRef{
				Schema: strings.ToLower(rv.Schemaname),
				Name:   strings.ToLower(rv.Relname),
			})
			continue
		}
		if join := node.GetJoinExpr(); join != nil {
			if join.Larg != nil {
				extractTablesFromClause([]*pg_query.Node{join.Larg}, tables)
			}
			if join.Rarg != nil {
				extractTablesFromClause([]*pg_query.Node{join.Rarg}, tables)
			}
		}
	}
}

var (
	reForUpdate = regexp.MustCompile(`(?i)\bFOR\s+(?:NO\s+KEY\s+)?UPDATE\b`)
	reFromTable = regexp.MustCompile(`(?i)\bFROM\s+([a-zA-Z0-9_.]+)`)
)

func heuristicExtractLockedTables(sql string) []TableRef {
	cleanSQL := reStringLiteral.ReplaceAllString(sql, "''")
	if !reForUpdate.MatchString(cleanSQL) {
		return nil
	}

	matches := reFromTable.FindStringSubmatch(cleanSQL)
	if len(matches) > 1 {
		raw := strings.ToLower(strings.Trim(matches[1], "\""))
		parts := strings.Split(raw, ".")
		if len(parts) == 2 {
			return []TableRef{{Schema: parts[0], Name: parts[1]}}
		}
		return []TableRef{{Name: parts[0]}}
	}
	return nil
}

// AdvisoryProtectsTable checks if an advisory lock name specifically correlates to the target critical table.
func AdvisoryProtectsTable(lockIdentifier string, table TableRef) bool {
	lowerLock := strings.ToLower(lockIdentifier)
	lowerTable := strings.ToLower(table.Name)

	if strings.Contains(lowerLock, lowerTable) {
		return true
	}
	// Check standard aliases/domains (e.g. "saldo" -> "balances", "rekening" -> "accounts")
	switch lowerTable {
	case "balances", "saldo":
		return strings.Contains(lowerLock, "balances") || strings.Contains(lowerLock, "saldo")
	case "accounts", "rekening":
		return strings.Contains(lowerLock, "accounts") || strings.Contains(lowerLock, "rekening")
	case "inventory", "kuota":
		return strings.Contains(lowerLock, "inventory") || strings.Contains(lowerLock, "kuota")
	}
	return false
}

// ExtractAdvisoryLockTarget extracts any table or domain targeted by an inline SQL advisory lock query.
func ExtractAdvisoryLockTarget(sql string) string {
	lower := strings.ToLower(sql)
	if !strings.Contains(lower, "pg_advisory_xact_lock") && !strings.Contains(lower, "pg_try_advisory_xact_lock") {
		return ""
	}
	return lower
}

// isTableProtected verifies if target critical table is protected by a matching row lock or correlated advisory lock.
func isTableProtected(target TableRef, lockedTables []TableRef, advisoryCalls []string) bool {
	for _, lt := range lockedTables {
		if target.Matches(lt) {
			return true
		}
	}
	for _, adv := range advisoryCalls {
		if AdvisoryProtectsTable(adv, target) {
			return true
		}
	}
	return false
}

func isEnclosedInCorrelatedAdvisory(call *ast.CallExpr, body *ast.BlockStmt, table TableRef) bool {
	var enclosed bool
	ast.Inspect(body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := getCallTargetName(c.Fun)
		if name == "WithAdvisoryLock" || strings.HasSuffix(name, ".WithAdvisoryLock") {
			var lockArg string
			for _, arg := range c.Args {
				if callsite.IsContextArg(arg, nil) {
					continue
				}
				if _, isFunc := arg.(*ast.FuncLit); isFunc {
					continue
				}
				if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					lockArg = strings.Trim(lit.Value, "`\"")
					break
				} else if id, ok := arg.(*ast.Ident); ok {
					lockArg = id.Name
					if body != nil {
						ast.Inspect(body, func(in ast.Node) bool {
							if as, ok := in.(*ast.AssignStmt); ok {
								for i, lhs := range as.Lhs {
									if lid, ok := lhs.(*ast.Ident); ok && lid.Name == id.Name && i < len(as.Rhs) {
										if slit, ok := as.Rhs[i].(*ast.BasicLit); ok && slit.Kind == token.STRING {
											lockArg = strings.Trim(slit.Value, "`\"")
										}
									}
								}
							}
							return true
						})
					}
					break
				}
			}
			if !AdvisoryProtectsTable(lockArg, table) {
				return true
			}

			for _, arg := range c.Args {
				if lit, ok := arg.(*ast.FuncLit); ok && lit.Body != nil {
					ast.Inspect(lit.Body, func(in ast.Node) bool {
						if in == call {
							enclosed = true
							return false
						}
						return true
					})
				}
			}
		}
		return true
	})
	return enclosed
}
