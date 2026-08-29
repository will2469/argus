// Package a26_like_sanitize inspects SQL queries to identify parameter placeholders bound to LIKE/ILIKE clauses.
package a26_like_sanitize

import (
	"regexp"
	"strconv"
	"strings"

	pgquery "github.com/pganalyze/pg_query_go/v6"
)

var paramRegex = regexp.MustCompile(`\$(\d+)`)

// FindLikeParamIndices analyzes an SQL query string and returns the 1-based parameter indices ($N)
// that are used as pattern expressions in LIKE or ILIKE clauses.
func FindLikeParamIndices(sql string) []int {
	if sql == "" {
		return nil
	}

	normSQL := strings.ToUpper(sql)
	if !strings.Contains(normSQL, "LIKE") && !strings.Contains(normSQL, "~~") {
		return nil
	}

	result, err := pgquery.Parse(sql)
	if err != nil {
		return findLikeParamIndicesRegex(sql)
	}

	var indices []int
	for _, rawStmt := range result.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}
		collectLikeParamsFromNode(rawStmt.Stmt, &indices)
	}

	if len(indices) == 0 {
		return findLikeParamIndicesRegex(sql)
	}

	return deduplicate(indices)
}

func collectLikeParamsFromNode(node *pgquery.Node, indices *[]int) {
	if node == nil {
		return
	}

	if sel := node.GetSelectStmt(); sel != nil {
		collectLikeParamsFromWhere(sel.WhereClause, indices)
		for _, fromItem := range sel.FromClause {
			collectLikeParamsFromNode(fromItem, indices)
		}
	} else if upd := node.GetUpdateStmt(); upd != nil {
		collectLikeParamsFromWhere(upd.WhereClause, indices)
	} else if del := node.GetDeleteStmt(); del != nil {
		collectLikeParamsFromWhere(del.WhereClause, indices)
	} else if join := node.GetJoinExpr(); join != nil {
		collectLikeParamsFromWhere(join.Quals, indices)
		collectLikeParamsFromNode(join.Larg, indices)
		collectLikeParamsFromNode(join.Rarg, indices)
	}
}

func collectLikeParamsFromWhere(whereNode *pgquery.Node, indices *[]int) {
	if whereNode == nil {
		return
	}

	if aexpr := whereNode.GetAExpr(); aexpr != nil {
		if isLikeExpr(aexpr) {
			extractParamsFromPattern(aexpr.Rexpr, indices)
		} else {
			collectLikeParamsFromWhere(aexpr.Lexpr, indices)
			collectLikeParamsFromWhere(aexpr.Rexpr, indices)
		}
	} else if boolExpr := whereNode.GetBoolExpr(); boolExpr != nil {
		for _, arg := range boolExpr.Args {
			collectLikeParamsFromWhere(arg, indices)
		}
	}
}

func isLikeExpr(aexpr *pgquery.A_Expr) bool {
	if aexpr == nil {
		return false
	}
	if aexpr.Kind == pgquery.A_Expr_Kind_AEXPR_LIKE ||
		aexpr.Kind == pgquery.A_Expr_Kind_AEXPR_ILIKE ||
		aexpr.Kind == pgquery.A_Expr_Kind_AEXPR_SIMILAR {
		return true
	}
	for _, nameNode := range aexpr.Name {
		if s, ok := nameNode.Node.(*pgquery.Node_String_); ok {
			strVal := s.String_.GetSval()
			upper := strings.ToUpper(strVal)
			if upper == "~~" || upper == "~~*" || upper == "!~~" || upper == "!~~*" ||
				strings.Contains(upper, "LIKE") || strings.Contains(upper, "ILIKE") {
				return true
			}
		}
	}
	return false
}

func extractParamsFromPattern(patternNode *pgquery.Node, indices *[]int) {
	if patternNode == nil {
		return
	}

	if param := patternNode.GetParamRef(); param != nil && param.Number > 0 {
		*indices = append(*indices, int(param.Number))
	} else if aexpr := patternNode.GetAExpr(); aexpr != nil {
		extractParamsFromPattern(aexpr.Lexpr, indices)
		extractParamsFromPattern(aexpr.Rexpr, indices)
	} else if typeCast := patternNode.GetTypeCast(); typeCast != nil {
		extractParamsFromPattern(typeCast.Arg, indices)
	} else if funcCall := patternNode.GetFuncCall(); funcCall != nil {
		for _, arg := range funcCall.Args {
			extractParamsFromPattern(arg, indices)
		}
	} else if coalesce := patternNode.GetCoalesceExpr(); coalesce != nil {
		for _, arg := range coalesce.Args {
			extractParamsFromPattern(arg, indices)
		}
	}
}

func findLikeParamIndicesRegex(sql string) []int {
	var indices []int
	likeRegex := regexp.MustCompile(`(?i)(?:I?LIKE|~~|~~[*])\s+([^AND|OR|\)|;]+)`)
	matches := likeRegex.FindAllStringSubmatch(sql, -1)
	for _, m := range matches {
		if len(m) > 1 {
			paramMatches := paramRegex.FindAllStringSubmatch(m[1], -1)
			for _, pm := range paramMatches {
				if len(pm) > 1 {
					if num, err := strconv.Atoi(pm[1]); err == nil && num > 0 {
						indices = append(indices, num)
					}
				}
			}
		}
	}
	return deduplicate(indices)
}

func deduplicate(in []int) []int {
	seen := make(map[int]bool)
	var out []int
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
