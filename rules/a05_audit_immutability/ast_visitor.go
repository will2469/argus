// Package a05_audit_immutability provides recursive SQL AST traversal to detect
// mutations on immutable audit log tables across statements and CTEs, with schema qualification.
package a05_audit_immutability

import (
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/will2469/argus/shared/sqlparser"
)

// CheckSQLTampering inspects a SQL query string for any forbidden mutations on audit tables,
// traversing root statements and nested Common Table Expressions (CTEs).
func CheckSQLTampering(query string, auditTables map[string]bool) (op string, table string) {
	tree, err := sqlparser.Parse(query)
	if err != nil {
		return "", ""
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}
		if op, tbl := visitNode(rawStmt.Stmt, auditTables); op != "" {
			return op, tbl
		}
	}
	return "", ""
}

func isTargetAuditTable(schemaname, relname string, auditTables map[string]bool) bool {
	if auditTables == nil {
		return false
	}
	rel := strings.ToLower(strings.TrimSpace(relname))
	schema := strings.ToLower(strings.TrimSpace(schemaname))

	if schema != "" {
		qualified := schema + "." + rel
		if auditTables[qualified] {
			return true
		}
		if schema == "public" && auditTables[rel] {
			return true
		}
		return false
	}

	if auditTables[rel] || auditTables["public."+rel] {
		return true
	}
	return false
}

func visitNode(node *pg_query.Node, auditTables map[string]bool) (string, string) {
	if node == nil {
		return "", ""
	}

	// 1. UPDATE
	if u := node.GetUpdateStmt(); u != nil {
		if u.Relation != nil && isTargetAuditTable(u.Relation.Schemaname, u.Relation.Relname, auditTables) {
			return "UPDATE", formatTableName(u.Relation.Schemaname, u.Relation.Relname)
		}
		if op, tbl := visitWithClause(u.WithClause, auditTables); op != "" {
			return op, tbl
		}
	}

	// 2. DELETE
	if d := node.GetDeleteStmt(); d != nil {
		if d.Relation != nil && isTargetAuditTable(d.Relation.Schemaname, d.Relation.Relname, auditTables) {
			return "DELETE", formatTableName(d.Relation.Schemaname, d.Relation.Relname)
		}
		if op, tbl := visitWithClause(d.WithClause, auditTables); op != "" {
			return op, tbl
		}
	}

	// 3. TRUNCATE
	if t := node.GetTruncateStmt(); t != nil {
		for _, r := range t.Relations {
			if rv := r.GetRangeVar(); rv != nil && isTargetAuditTable(rv.Schemaname, rv.Relname, auditTables) {
				return "TRUNCATE", formatTableName(rv.Schemaname, rv.Relname)
			}
		}
	}

	// 4. MERGE (PostgreSQL 17+)
	if m := node.GetMergeStmt(); m != nil {
		if m.Relation != nil && isTargetAuditTable(m.Relation.Schemaname, m.Relation.Relname, auditTables) {
			return "MERGE", formatTableName(m.Relation.Schemaname, m.Relation.Relname)
		}
		if op, tbl := visitWithClause(m.WithClause, auditTables); op != "" {
			return op, tbl
		}
	}

	// 5. DROP TABLE
	if dr := node.GetDropStmt(); dr != nil {
		for _, obj := range dr.Objects {
			if list := obj.GetList(); list != nil {
				var parts []string
				for _, item := range list.Items {
					if s := item.GetString_(); s != nil {
						parts = append(parts, s.Sval)
					}
				}
				var schema, table string
				if len(parts) == 1 {
					table = parts[0]
				} else if len(parts) >= 2 {
					schema = parts[len(parts)-2]
					table = parts[len(parts)-1]
				}
				if isTargetAuditTable(schema, table, auditTables) {
					return "DROP", formatTableName(schema, table)
				}
			}
		}
	}

	// 6. SELECT (check CTEs)
	if s := node.GetSelectStmt(); s != nil {
		if op, tbl := visitWithClause(s.WithClause, auditTables); op != "" {
			return op, tbl
		}
	}

	// 7. INSERT (check CTEs and source select)
	if i := node.GetInsertStmt(); i != nil {
		if op, tbl := visitWithClause(i.WithClause, auditTables); op != "" {
			return op, tbl
		}
		if i.SelectStmt != nil {
			if op, tbl := visitNode(i.SelectStmt, auditTables); op != "" {
				return op, tbl
			}
		}
	}

	return "", ""
}

func formatTableName(schema, rel string) string {
	rel = strings.ToLower(strings.TrimSpace(rel))
	schema = strings.ToLower(strings.TrimSpace(schema))
	if schema != "" && schema != "public" {
		return schema + "." + rel
	}
	return rel
}

func visitWithClause(wc *pg_query.WithClause, auditTables map[string]bool) (string, string) {
	if wc == nil {
		return "", ""
	}
	for _, cteNode := range wc.Ctes {
		cte := cteNode.GetCommonTableExpr()
		if cte == nil || cte.Ctequery == nil {
			continue
		}
		if op, tbl := visitNode(cte.Ctequery, auditTables); op != "" {
			return op, tbl
		}
	}
	return "", ""
}
