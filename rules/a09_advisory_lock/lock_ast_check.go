// Package a09_advisory_lock provides AST inspection of PostgreSQL advisory lock functions,
// distinguishing session-level locks from transaction-scoped locks, and respecting 1-key vs 2-key
// (namespace class ID vs resource object ID) locking semantics.
package a09_advisory_lock

import (
	"regexp"
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

// InspectAdvisorySQL parses a SQL string and checks for forbidden session locks or unnamespaced hardcoded keys.
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

			// 1. Check for session-level lock functions (forbidden on connection pools)
			if sessionLockFuncs[lowerName] {
				violations = append(violations, AdvisoryViolation{
					Type:     ViolationSessionLock,
					FuncName: fnName,
				})
				continue
			}

			// 2. Check for xact lock key semantics
			if xactLockFuncs[lowerName] {
				if v := checkXactLockArgs(fnName, fn.Args); v != nil {
					violations = append(violations, *v)
				}
			}
		}
	}

	return violations
}

func checkXactLockArgs(fnName string, args []*pg_query.Node) *AdvisoryViolation {
	if len(args) == 0 {
		return nil
	}

	// 1-Argument Form: pg_advisory_xact_lock(key bigint)
	// A single raw integer literal is un-namespaced across the entire database.
	if len(args) == 1 {
		if isIntegerConstant(args[0]) {
			return &AdvisoryViolation{
				Type:     ViolationHardcodedIntKey,
				FuncName: fnName,
			}
		}
		return nil
	}

	// 2-Argument Form: pg_advisory_xact_lock(classid int, objid int)
	// PostgreSQL separates the 64-bit space into (classid/namespace, objid/resource).
	// - classid (arg 0) may be a constant integer namespace ID.
	// - objid (arg 1) must be a dynamic or bound resource parameter ($1, column, etc.).
	// If the resource (arg 1) is a hardcoded literal, or both are literals, it is an unnamespaced magic number.
	if len(args) >= 2 {
		arg1IsInt := isIntegerConstant(args[0])
		arg2IsInt := isIntegerConstant(args[1])

		if arg1IsInt && arg2IsInt {
			// Both are hardcoded magic integers: e.g. pg_advisory_xact_lock(1, 2)
			return &AdvisoryViolation{
				Type:     ViolationHardcodedIntKey,
				FuncName: fnName,
			}
		}
		if arg2IsInt {
			// Resource object ID is hardcoded magic integer: e.g. pg_advisory_xact_lock($1, 42)
			return &AdvisoryViolation{
				Type:     ViolationHardcodedIntKey,
				FuncName: fnName,
			}
		}
		// If arg1 is an integer constant and arg2 is a dynamic parameter ($1), this is VALID!
	}

	return nil
}

func isIntegerConstant(node *pg_query.Node) bool {
	if node == nil {
		return false
	}
	if c := node.GetAConst(); c != nil {
		if c.GetIval() != nil {
			return true
		}
	}
	return false
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

var (
	re1ArgMagic = regexp.MustCompile(`(?i)pg_(?:try_)?advisory_xact_lock(?:_shared)?\s*\(\s*\d+\s*\)`)
	re2ArgMagic = regexp.MustCompile(`(?i)pg_(?:try_)?advisory_xact_lock(?:_shared)?\s*\(\s*[^,]+,\s*\d+\s*\)`)
)

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

	if re1ArgMagic.MatchString(sql) || re2ArgMagic.MatchString(sql) {
		violations = append(violations, AdvisoryViolation{
			Type:     ViolationHardcodedIntKey,
			FuncName: "pg_advisory_xact_lock",
		})
	}

	return violations
}
