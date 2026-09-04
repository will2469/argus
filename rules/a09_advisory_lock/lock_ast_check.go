// Package a09_advisory_lock provides AST inspection of PostgreSQL advisory lock functions,
// distinguishing session-level locks from transaction-scoped locks, and respecting 1-key vs 2-key
// (namespace class ID vs resource object ID) locking semantics.
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

// InspectAdvisorySQL parses a SQL string and checks for forbidden session locks or unnamespaced hardcoded keys.
func InspectAdvisorySQL(sql string) []AdvisoryViolation {
	tree, err := sqlparser.Parse(sql)
	if err != nil {
		return nil
	}

	var violations []AdvisoryViolation
	for _, stmt := range tree.Stmts {
		if stmt.Stmt == nil {
			continue
		}
		if sel := stmt.Stmt.GetSelectStmt(); sel != nil {
			inspectSelectStmt(sel, &violations)
		}
	}

	return violations
}

func inspectSelectStmt(sel *pg_query.SelectStmt, violations *[]AdvisoryViolation) {
	if sel == nil {
		return
	}

	// 1. Check TargetList
	for _, target := range sel.TargetList {
		if res := target.GetResTarget(); res != nil && res.Val != nil {
			if fn := res.Val.GetFuncCall(); fn != nil {
				if v := checkFuncCall(fn); v != nil {
					*violations = append(*violations, *v)
				}
			}
		}
	}

	// 2. Check WhereClause
	if sel.WhereClause != nil {
		if fn := sel.WhereClause.GetFuncCall(); fn != nil {
			if v := checkFuncCall(fn); v != nil {
				*violations = append(*violations, *v)
			}
		}
	}

	// 3. Check CTEs in WithClause
	if sel.WithClause != nil {
		for _, cteNode := range sel.WithClause.Ctes {
			if cte := cteNode.GetCommonTableExpr(); cte != nil && cte.Ctequery != nil {
				if cteSel := cte.Ctequery.GetSelectStmt(); cteSel != nil {
					inspectSelectStmt(cteSel, violations)
				}
			}
		}
	}

	// 4. Check Set Operations (UNION, INTERSECT, EXCEPT)
	if sel.Larg != nil {
		inspectSelectStmt(sel.Larg, violations)
	}
	if sel.Rarg != nil {
		inspectSelectStmt(sel.Rarg, violations)
	}
}

func checkFuncCall(fn *pg_query.FuncCall) *AdvisoryViolation {
	if fn == nil {
		return nil
	}
	fnName := extractFuncName(fn)
	lowerName := strings.ToLower(fnName)

	if sessionLockFuncs[lowerName] {
		return &AdvisoryViolation{
			Type:     ViolationSessionLock,
			FuncName: fnName,
		}
	}
	if xactLockFuncs[lowerName] {
		return checkXactLockArgs(fnName, fn.Args)
	}
	return nil
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
			return &AdvisoryViolation{
				Type:     ViolationHardcodedIntKey,
				FuncName: fnName,
			}
		}
		if arg2IsInt {
			return &AdvisoryViolation{
				Type:     ViolationHardcodedIntKey,
				FuncName: fnName,
			}
		}
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
