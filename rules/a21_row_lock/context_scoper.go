// Package a21_row_lock identifies single-entity point lookups and loads key columns for ARGUS-A21.
package a21_row_lock

import (
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/will2469/argus/shared/config"
)

var defaultKeyColumns = []string{
	"id",
	"uuid",
	"pk",
	"key",
}

// GetKeyColumns retrieves the set of configured point-lookup / primary key columns.
func GetKeyColumns(cfg *config.Config) map[string]bool {
	keyMap := make(map[string]bool)

	for _, k := range defaultKeyColumns {
		keyMap[strings.ToLower(k)] = true
	}

	if cfg == nil {
		return keyMap
	}

	custom := cfg.GetStringSlice(RuleCode, "point_lookup_columns", nil)
	if len(custom) == 0 {
		custom = cfg.GetStringSlice(RuleCode, "key_columns", nil)
	}
	for _, k := range custom {
		if trimmed := strings.TrimSpace(k); trimmed != "" {
			keyMap[strings.ToLower(trimmed)] = true
		}
	}
	return keyMap
}

// IsPointLookup checks if the WHERE clause contains an equality check on a primary or key column.
func IsPointLookup(sel *pg_query.SelectStmt, keyColumnMap map[string]bool) bool {
	if sel == nil || sel.WhereClause == nil {
		return false
	}
	return checkExprForPointLookup(sel.WhereClause, keyColumnMap)
}

func checkExprForPointLookup(node *pg_query.Node, keyColumnMap map[string]bool) bool {
	if node == nil {
		return false
	}

	if aExpr := node.GetAExpr(); aExpr != nil {
		switch aExpr.Kind {
		case pg_query.A_Expr_Kind_AEXPR_OP:
			opName := ""
			for _, nameNode := range aExpr.Name {
				if str := nameNode.GetString_(); str != nil {
					opName = str.Sval
				}
			}
			if opName == "=" {
				if isKeyColumnRef(aExpr.Lexpr, keyColumnMap) || isKeyColumnRef(aExpr.Rexpr, keyColumnMap) {
					return true
				}
			}
		}
	}

	if boolExpr := node.GetBoolExpr(); boolExpr != nil {
		if boolExpr.Boolop == pg_query.BoolExprType_AND_EXPR {
			for _, arg := range boolExpr.Args {
				if checkExprForPointLookup(arg, keyColumnMap) {
					return true
				}
			}
		}
	}

	return false
}

func isKeyColumnRef(node *pg_query.Node, keyColumnMap map[string]bool) bool {
	if node == nil {
		return false
	}
	colRef := node.GetColumnRef()
	if colRef == nil {
		return false
	}
	for _, field := range colRef.Fields {
		if str := field.GetString_(); str != nil {
			lower := strings.ToLower(str.Sval)
			if keyColumnMap[lower] {
				return true
			}
		}
	}
	return false
}
