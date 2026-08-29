// Package a06_runtime_ddl provides comprehensive detection of PostgreSQL DDL AST nodes.
package a06_runtime_ddl

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/will2469/argus/shared/sqlparser"
)

// DetectDDLFromAST parses a SQL string and checks if any statement is a DDL operation.
// It inspects all statements in multi-statement queries (e.g. "SELECT 1; DROP TABLE users;").
func DetectDDLFromAST(query string) string {
	tree, err := sqlparser.Parse(query)
	if err != nil {
		return ""
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}
		if op := IsDDLNode(rawStmt.Stmt); op != "" {
			return op
		}
	}
	return ""
}

// IsDDLNode checks whether a given AST node represents a DDL statement.
func IsDDLNode(stmt *pg_query.Node) string {
	if stmt == nil {
		return ""
	}

	if stmt.GetCreateStmt() != nil {
		return "CREATE TABLE"
	}
	if stmt.GetDropStmt() != nil {
		return "DROP"
	}
	if stmt.GetAlterTableStmt() != nil {
		return "ALTER TABLE"
	}
	if stmt.GetTruncateStmt() != nil {
		return "TRUNCATE"
	}
	if stmt.GetGrantStmt() != nil {
		return "GRANT/REVOKE"
	}
	if stmt.GetGrantRoleStmt() != nil {
		return "GRANT ROLE"
	}
	if stmt.GetIndexStmt() != nil {
		return "CREATE INDEX"
	}
	if stmt.GetRenameStmt() != nil {
		return "RENAME"
	}
	if stmt.GetCommentStmt() != nil {
		return "COMMENT"
	}
	if stmt.GetCreateSeqStmt() != nil {
		return "CREATE SEQUENCE"
	}
	if stmt.GetAlterSeqStmt() != nil {
		return "ALTER SEQUENCE"
	}
	if stmt.GetCreateSchemaStmt() != nil {
		return "CREATE SCHEMA"
	}
	if stmt.GetCreateExtensionStmt() != nil {
		return "CREATE EXTENSION"
	}
	if stmt.GetViewStmt() != nil {
		return "CREATE VIEW"
	}
	if stmt.GetCreateTrigStmt() != nil {
		return "CREATE TRIGGER"
	}
	if stmt.GetCreateFunctionStmt() != nil {
		return "CREATE FUNCTION"
	}
	if stmt.GetDoStmt() != nil {
		return "DO (ANONYMOUS PL/pgSQL)"
	}

	return ""
}
