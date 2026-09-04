// Package a05_audit_immutability provides skeletal DML prefix and table extraction
// for dynamic SQL queries constructed via concatenation or string builders.
package a05_audit_immutability

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"github.com/will2469/argus/shared/callsite"
)

var dmlPrefixRegex = regexp.MustCompile(`(?i)^\s*(UPDATE|DELETE\s+FROM|INSERT\s+INTO|MERGE\s+INTO)\s+([a-zA-Z0-9_.]+)`)

// ExtractSkeletalDML extracts the DML operation and target table from an AST call expression
// if a static prefix is present (e.g. nested BinaryExpr concat, strings.Builder.WriteString, Sprintf).
func (t *flowTracker) ExtractSkeletalDML(call *ast.CallExpr) (op, table string, ok bool) {
	if call == nil || t == nil {
		return "", "", false
	}

	sqlArg := callsite.ExtractSQLArg(call, t.pass)
	if sqlArg == nil {
		return "", "", false
	}

	leadingStr := t.extractLeadingStaticString(sqlArg, call.Pos(), make(map[string]bool), 0)
	if leadingStr == "" {
		return "", "", false
	}

	match := dmlPrefixRegex.FindStringSubmatch(leadingStr)
	if len(match) < 3 {
		return "", "", false
	}

	rawOp := strings.ToUpper(strings.TrimSpace(match[1]))
	rawTable := strings.TrimSpace(match[2])

	// If the extracted table name contains placeholder format specifiers (e.g. %s), it is dynamic
	if strings.Contains(rawTable, "%") || rawTable == "" {
		return "", "", false
	}

	// Canonicalize operation name
	canonicalOp := rawOp
	fields := strings.Fields(rawOp)
	if len(fields) > 0 {
		canonicalOp = fields[0]
	}

	return canonicalOp, rawTable, true
}

func (t *flowTracker) extractLeadingStaticString(expr ast.Expr, beforePos token.Pos, visited map[string]bool, depth int) string {
	if expr == nil || depth > 10 {
		return ""
	}

	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			return unquoteLiteral(e.Value)
		}
	case *ast.ParenExpr:
		return t.extractLeadingStaticString(e.X, beforePos, visited, depth+1)
	case *ast.BinaryExpr:
		if e.Op == token.ADD {
			return t.extractLeadingStaticString(e.X, beforePos, visited, depth+1)
		}
	case *ast.Ident:
		if visited[e.Name] {
			return ""
		}
		newVisited := make(map[string]bool, len(visited)+1)
		for k, v := range visited {
			newVisited[k] = v
		}
		newVisited[e.Name] = true
		return t.findIdentLeadingString(e, beforePos, newVisited, depth+1)
	case *ast.CallExpr:
		return t.extractCallLeadingString(e, beforePos, visited, depth+1)
	}

	return ""
}

func (t *flowTracker) findIdentLeadingString(id *ast.Ident, beforePos token.Pos, visited map[string]bool, depth int) string {
	if id == nil || depth > 10 {
		return ""
	}

	// Check if declared as const in file
	pkgVals := t.findPackageDeclValues(id.Name, 0)
	for v := range pkgVals {
		if v != "" {
			return v
		}
	}

	// Search backwards in function body for assignments strictly before beforePos
	if t.fn != nil && t.fn.Body != nil {
		var candidateRhs ast.Expr
		var assignPos token.Pos
		ast.Inspect(t.fn.Body, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			assign, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			if beforePos != token.NoPos && assign.Pos() >= beforePos {
				return false
			}
			for i, lhs := range assign.Lhs {
				if lhsIdent, isId := lhs.(*ast.Ident); isId && lhsIdent.Name == id.Name {
					if i < len(assign.Rhs) {
						candidateRhs = assign.Rhs[i]
						assignPos = assign.Pos()
					}
				}
			}
			return true
		})

		if candidateRhs != nil {
			return t.extractLeadingStaticString(candidateRhs, assignPos, visited, depth+1)
		}
	}

	return ""
}

func (t *flowTracker) extractCallLeadingString(call *ast.CallExpr, beforePos token.Pos, visited map[string]bool, depth int) string {
	if call == nil || depth > 10 {
		return ""
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	// Case 1: fmt.Sprintf
	if sel.Sel.Name == "Sprintf" && len(call.Args) > 0 {
		return t.extractLeadingStaticString(call.Args[0], beforePos, visited, depth+1)
	}

	// Case 2: strings.Builder.String() or bytes.Buffer.String()
	if sel.Sel.Name == "String" {
		targetIdent := getBuilderRootIdent(sel.X)
		if targetIdent != nil {
			return t.findFirstBuilderWriteString(targetIdent, beforePos, visited, depth+1)
		}
	}

	return ""
}

func (t *flowTracker) findFirstBuilderWriteString(targetIdent *ast.Ident, beforePos token.Pos, visited map[string]bool, depth int) string {
	if targetIdent == nil || t.fn == nil || t.fn.Body == nil || depth > 10 {
		return ""
	}

	var firstWriteLit string
	ast.Inspect(t.fn.Body, func(n ast.Node) bool {
		if firstWriteLit != "" {
			return false
		}
		if n == nil || (beforePos != token.NoPos && n.Pos() >= beforePos) {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		callSel, isSel := call.Fun.(*ast.SelectorExpr)
		if !isSel {
			return true
		}

		if (callSel.Sel.Name == "WriteString" || callSel.Sel.Name == "Write") && len(call.Args) > 0 {
			recvIdent := getBuilderRootIdent(callSel.X)
			if recvIdent != nil && recvIdent.Name == targetIdent.Name {
				lit := t.extractLeadingStaticString(call.Args[0], call.Pos(), visited, depth+1)
				if lit != "" {
					firstWriteLit = lit
					return false
				}
			}
		}
		return true
	})

	return firstWriteLit
}

func getBuilderRootIdent(expr ast.Expr) *ast.Ident {
	switch e := expr.(type) {
	case *ast.Ident:
		return e
	case *ast.UnaryExpr:
		return getBuilderRootIdent(e.X)
	case *ast.ParenExpr:
		return getBuilderRootIdent(e.X)
	}
	return nil
}

func isTargetAuditTableFromQualified(tableName string, auditTables map[string]bool) bool {
	if tableName == "" || auditTables == nil {
		return false
	}
	if strings.Contains(tableName, ".") {
		parts := strings.Split(tableName, ".")
		return isTargetAuditTable(parts[0], parts[1], auditTables)
	}
	return isTargetAuditTable("", tableName, auditTables)
}

func formatTableNameFromQualified(tbl string) string {
	tbl = strings.ToLower(strings.TrimSpace(tbl))
	if strings.HasPrefix(tbl, "public.") {
		return strings.TrimPrefix(tbl, "public.")
	}
	return tbl
}
