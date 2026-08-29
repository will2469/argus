// Package a09_advisory_lock provides AST inspection of PostgreSQL advisory lock functions,
// ensuring that applications use transaction-scoped locks with properly namespaced identifiers.
package a09_advisory_lock

import (
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/sqlparser"
)

type AdvisoryViolationType int

const (
	ViolationNone AdvisoryViolationType = iota
	ViolationSessionLock
	ViolationHardcodedIntKey
)

type AdvisoryViolation struct {
	Type     AdvisoryViolationType
	FuncName string
}

var sessionLockFuncs = map[string]bool{
	"pg_advisory_lock":            true,
	"pg_advisory_lock_shared":     true,
	"pg_try_advisory_lock":        true,
	"pg_try_advisory_lock_shared": true,
	"pg_advisory_unlock":          true,
	"pg_advisory_unlock_shared":   true,
	"pg_advisory_unlock_all":      true,
}

var xactLockFuncs = map[string]bool{
	"pg_advisory_xact_lock":            true,
	"pg_advisory_xact_lock_shared":     true,
	"pg_try_advisory_xact_lock":        true,
	"pg_try_advisory_xact_lock_shared": true,
}

// InspectAdvisorySQL parses a SQL string and checks for forbidden session locks or hardcoded keys.
func InspectAdvisorySQL(sql string) []AdvisoryViolation {
	tree, err := sqlparser.Parse(sql)
	if err != nil {
		return checkSubstrings(sql)
	}

	var violations []AdvisoryViolation

	for _, stmt := range tree.Stmts {
		if stmt.Stmt == nil {
			continue
		}
		sel := stmt.Stmt.GetSelectStmt()
		if sel == nil {
			continue
		}

		for _, target := range sel.TargetList {
			res := target.GetResTarget()
			if res == nil || res.Val == nil {
				continue
			}
			fn := res.Val.GetFuncCall()
			if fn == nil {
				continue
			}

			fnName := extractFuncName(fn)
			lowerName := strings.ToLower(fnName)

			// 1. Check for session-level lock functions
			if sessionLockFuncs[lowerName] {
				violations = append(violations, AdvisoryViolation{
					Type:     ViolationSessionLock,
					FuncName: fnName,
				})
				continue
			}

			// 2. Check for hardcoded integer literals in xact lock functions
			if xactLockFuncs[lowerName] {
				for _, arg := range fn.Args {
					if c := arg.GetAConst(); c != nil {
						if c.GetIval() != nil {
							violations = append(violations, AdvisoryViolation{
								Type:     ViolationHardcodedIntKey,
								FuncName: fnName,
							})
							break
						}
					}
				}
			}
		}
	}

	return violations
}

func extractFuncName(fn *pg_query.FuncCall) string {
	if len(fn.Funcname) == 0 {
		return ""
	}
	last := fn.Funcname[len(fn.Funcname)-1]
	if str := last.GetString_(); str != nil {
		return str.Sval
	}
	return ""
}

func checkSubstrings(sql string) []AdvisoryViolation {
	var violations []AdvisoryViolation
	lower := strings.ToLower(sql)

	for fn := range sessionLockFuncs {
		if strings.Contains(lower, fn) {
			violations = append(violations, AdvisoryViolation{
				Type:     ViolationSessionLock,
				FuncName: fn,
			})
		}
	}

	for fn := range xactLockFuncs {
		if strings.Contains(lower, fn) {
			for _, digit := range []string{"(1)", "(2)", "(42)", "(100)", "(999)"} {
				if strings.Contains(lower, fn+digit) {
					violations = append(violations, AdvisoryViolation{
						Type:     ViolationHardcodedIntKey,
						FuncName: fn,
					})
					break
				}
			}
		}
	}

	return violations
}
