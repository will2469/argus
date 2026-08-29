// Package a04_orderby parses and validates SQL ORDER BY clauses and SortClause AST nodes.
package a04_orderby

import (
	"go/ast"
	"regexp"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/will2469/argus/shared/sqlparser"
)

var (
	orderByRegex    = regexp.MustCompile(`(?i)\bORDER\s+BY\b`)
	clauseEndRegex  = regexp.MustCompile(`(?i)\b(LIMIT|OFFSET|FETCH|FOR|UNION|INTERSECT|EXCEPT|WINDOW)\b|[);]`)
	formatVerbRegex = regexp.MustCompile(`%[+\-# 0]*[0-9]*(\.[0-9]+)?[a-zA-Z%]`)
)

// SortClauseInfo represents a parsed PostgreSQL SortClause node.
type SortClauseInfo struct {
	ColumnName string
	Direction  string
	IsComplex  bool
}

// IsFmtSprintf determines if an AST call expression is fmt.Sprintf.
func IsFmtSprintf(call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Sprintf" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "fmt"
}

// HasOrderByClause checks whether the format string contains an ORDER BY clause.
func HasOrderByClause(format string) bool {
	return orderByRegex.MatchString(format)
}

// GetOrderByArgIndices returns 1-based argument indices of placeholders within the ORDER BY clause.
func GetOrderByArgIndices(format string) []int {
	loc := orderByRegex.FindStringIndex(format)
	if loc == nil {
		return nil
	}

	orderStart := loc[1]
	sub := format[orderStart:]
	endLoc := clauseEndRegex.FindStringIndex(sub)

	var orderClause string
	if endLoc != nil {
		orderClause = sub[:endLoc[0]]
	} else {
		orderClause = sub
	}

	prefix := format[:loc[0]]
	prefixVerbs := CountFormatVerbs(prefix)
	clauseVerbs := CountFormatVerbs(orderClause)

	var indices []int
	for i := 0; i < clauseVerbs; i++ {
		indices = append(indices, 1+prefixVerbs+i)
	}
	return indices
}

// CountFormatVerbs counts the number of format verbs (excluding escaped %%) in a format string.
func CountFormatVerbs(s string) int {
	matches := formatVerbRegex.FindAllString(s, -1)
	count := 0
	for _, m := range matches {
		if m != "%%" {
			count++
		}
	}
	return count
}

// IsQuotingSanitizer checks whether an expression calls pgx.Identifier.Sanitize or SanitizeIdentifier.
func IsQuotingSanitizer(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "Sanitize" || sel.Sel.Name == "SanitizeIdentifier"
}

// ExtractSortClauses parses a SQL string and extracts its SortClause definitions from the PostgreSQL AST.
func ExtractSortClauses(sqlText string) ([]SortClauseInfo, error) {
	parsed, err := sqlparser.Parse(sqlText)
	if err != nil {
		return nil, err
	}

	var results []SortClauseInfo
	for _, raw := range parsed.Stmts {
		stmtNode := raw.Stmt
		if stmtNode == nil {
			continue
		}
		sel := stmtNode.GetSelectStmt()
		if sel == nil || len(sel.SortClause) == 0 {
			continue
		}

		for _, item := range sel.SortClause {
			sortBy := item.GetSortBy()
			if sortBy == nil {
				continue
			}

			info := SortClauseInfo{
				Direction: "ASC",
			}
			if sortBy.SortbyDir == pg_query.SortByDir_SORTBY_DESC {
				info.Direction = "DESC"
			}

			if sortBy.Node != nil {
				if colRef := sortBy.Node.GetColumnRef(); colRef != nil {
					var parts []string
					for _, f := range colRef.Fields {
						if str := f.GetString_(); str != nil {
							parts = append(parts, str.Sval)
						}
					}
					if len(parts) > 0 {
						info.ColumnName = parts[len(parts)-1]
					}
				} else {
					// Expression-based sort (e.g. CASE WHEN, SubLink, FuncCall)
					info.IsComplex = true
				}
			}

			results = append(results, info)
		}
	}

	return results, nil
}
