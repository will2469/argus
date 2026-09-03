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

	indices := parseAndCollectLikeParams(sql)
	if len(indices) > 0 {
		return deduplicate(indices)
	}

	// If direct AST parsing produced 0 indices, try wrapping SQL fragments
	trimmed := strings.TrimSpace(sql)
	upperTrimmed := strings.ToUpper(trimmed)
	var wrappers []string
	if strings.HasPrefix(upperTrimmed, "WHERE") {
		wrappers = append(wrappers, "SELECT 1 FROM __argus_dummy__ "+trimmed)
	} else if strings.HasPrefix(upperTrimmed, "AND") || strings.HasPrefix(upperTrimmed, "OR") {
		wrappers = append(wrappers, "SELECT 1 FROM __argus_dummy__ WHERE 1=1 "+trimmed)
	} else {
		wrappers = append(wrappers, "SELECT 1 FROM __argus_dummy__ WHERE "+trimmed)
	}

	for _, wrapped := range wrappers {
		if fragmentIndices := parseAndCollectLikeParams(wrapped); len(fragmentIndices) > 0 {
			return deduplicate(fragmentIndices)
		}
	}

	return findLikeParamIndicesRegex(sql)
}

func parseAndCollectLikeParams(sql string) []int {
	result, err := pgquery.Parse(sql)
	if err != nil {
		return nil
	}

	var indices []int
	for _, rawStmt := range result.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}
		collectLikeParamsFromNode(rawStmt.Stmt, &indices)
	}
	return indices
}

func collectLikeParamsFromNode(node *pgquery.Node, indices *[]int) {
	if node == nil {
		return
	}

	switch {
	case node.GetSelectStmt() != nil:
		sel := node.GetSelectStmt()
		collectLikeParamsFromWhere(sel.WhereClause, indices)
		for _, fromItem := range sel.FromClause {
			collectLikeParamsFromNode(fromItem, indices)
		}
	case node.GetUpdateStmt() != nil:
		collectLikeParamsFromWhere(node.GetUpdateStmt().WhereClause, indices)
	case node.GetDeleteStmt() != nil:
		collectLikeParamsFromWhere(node.GetDeleteStmt().WhereClause, indices)
	case node.GetJoinExpr() != nil:
		join := node.GetJoinExpr()
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
	switch {
	case patternNode.GetParamRef() != nil && patternNode.GetParamRef().Number > 0:
		*indices = append(*indices, int(patternNode.GetParamRef().Number))
	case patternNode.GetAExpr() != nil:
		extractParamsFromPattern(patternNode.GetAExpr().Lexpr, indices)
		extractParamsFromPattern(patternNode.GetAExpr().Rexpr, indices)
	case patternNode.GetTypeCast() != nil:
		extractParamsFromPattern(patternNode.GetTypeCast().Arg, indices)
	case patternNode.GetFuncCall() != nil:
		for _, arg := range patternNode.GetFuncCall().Args {
			extractParamsFromPattern(arg, indices)
		}
	case patternNode.GetCoalesceExpr() != nil:
		for _, arg := range patternNode.GetCoalesceExpr().Args {
			extractParamsFromPattern(arg, indices)
		}
	}
}

var (
	likeOpRegex    = regexp.MustCompile(`(?i)\b(?:I?LIKE|~~[*]?|!~~[*]?)\s+`)
	clauseBoundary = regexp.MustCompile(`^(?i)(?:AND|OR|GROUP|ORDER|HAVING|LIMIT|OFFSET|FETCH|RETURNING|UNION)\b`)
)

func findLikeParamIndicesRegex(sql string) []int {
	var indices []int
	clauses := extractLikePatternClauses(sql)
	for _, clause := range clauses {
		paramMatches := paramRegex.FindAllStringSubmatch(clause, -1)
		for _, pm := range paramMatches {
			if len(pm) > 1 {
				if num, err := strconv.Atoi(pm[1]); err == nil && num > 0 {
					indices = append(indices, num)
				}
			}
		}
	}
	return deduplicate(indices)
}

func extractLikePatternClauses(sql string) []string {
	var patterns []string
	locs := likeOpRegex.FindAllStringIndex(sql, -1)
	for _, loc := range locs {
		start := loc[1]
		end := scanPatternEnd(sql, start)
		if end > start {
			patterns = append(patterns, sql[start:end])
		}
	}
	return patterns
}

func scanPatternEnd(sql string, start int) int {
	n := len(sql)
	parenDepth := 0
	inQuote := false
	quoteChar := byte(0)

	for i := start; i < n; i++ {
		c := sql[i]

		if inQuote {
			if c == quoteChar {
				if i+1 < n && sql[i+1] == quoteChar {
					i++
					continue
				}
				inQuote = false
			}
			continue
		}

		if c == '\'' || c == '"' || c == '`' {
			inQuote, quoteChar = true, c
			continue
		}

		if c == '(' {
			parenDepth++
			continue
		}

		if c == ')' {
			if parenDepth > 0 {
				parenDepth--
				continue
			}
			return i
		}

		if parenDepth == 0 {
			if c == ';' {
				return i
			}
			if (i == start || isWhitespaceOrPunct(sql[i-1])) && clauseBoundary.MatchString(sql[i:]) {
				return i
			}
		}
	}
	return n
}

func isWhitespaceOrPunct(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == ')'
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
