package a15_ddl_grant

import (
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/directives"
	"github.com/will2469/argus/shared/migration"
	"github.com/will2469/argus/shared/sqlparser"
)

// CheckMigration inspects migration SQL for forbidden DDL grants or ownership changes to app roles.
func CheckMigration(filename, content string, dm *directives.DirectiveMap, reg *RoleRegistry) []migration.Issue {
	var issues []migration.Issue

	tree, err := sqlparser.Parse(content)
	if err != nil {
		return issues
	}

	for _, rawStmt := range tree.Stmts {
		if rawStmt.Stmt == nil {
			continue
		}

		// 1. Inspect GRANT statements
		if grantStmt := rawStmt.Stmt.GetGrantStmt(); grantStmt != nil && grantStmt.IsGrant {
			grantees := extractGrantees(grantStmt)
			for _, grantee := range grantees {
				if reg.IsAppRole(grantee) {
					isDDL, ddlPerms := extractDDLPermissions(grantStmt)
					if isDDL {
						line := migration.FindLineForKeyword(content, grantee)
						if dm != nil && (dm.IsLineIgnored(filename, line, RuleCode) || dm.IsLineIgnored(filename, line, RuleCode+".DDL-GRANT")) {
							continue
						}
						msg := fmt.Sprintf("Forbidden grant of DDL permission (%s) to runtime role %q; app roles must be DML-only",
							strings.Join(ddlPerms, ", "), grantee)
						issues = append(issues, migration.Issue{
							Rule:     RuleCode,
							Filename: filename,
							Line:     line,
							Message:  msg,
							Severity: "CRITICAL",
						})
					}
				}
			}
		}

		// 2. Inspect ALTER TABLE ... OWNER TO statements
		if alterStmt := rawStmt.Stmt.GetAlterTableStmt(); alterStmt != nil {
			for _, rawCmd := range alterStmt.Cmds {
				cmd := rawCmd.GetAlterTableCmd()
				if cmd != nil && cmd.Subtype == pg_query.AlterTableType_AT_ChangeOwner && cmd.Newowner != nil {
					newOwner := strings.ToLower(cmd.Newowner.Rolename)
					if reg.IsAppRole(newOwner) {
						line := migration.FindLineForKeyword(content, newOwner)
						if dm != nil && (dm.IsLineIgnored(filename, line, RuleCode) || dm.IsLineIgnored(filename, line, RuleCode+".OWNER-TO")) {
							continue
						}
						msg := fmt.Sprintf("Forbidden table ownership grant to runtime app role %q; ownership must be retained by admin/migrator", newOwner)
						issues = append(issues, migration.Issue{
							Rule:     RuleCode,
							Filename: filename,
							Line:     line,
							Message:  msg,
							Severity: "CRITICAL",
						})
					}
				}
			}
		}
	}

	return issues
}

func extractGrantees(stmt *pg_query.GrantStmt) []string {
	var grantees []string
	for _, g := range stmt.Grantees {
		if privRole := g.GetRoleSpec(); privRole != nil {
			if privRole.Roletype == pg_query.RoleSpecType_ROLESPEC_PUBLIC {
				grantees = append(grantees, "public")
			} else if privRole.Rolename != "" {
				grantees = append(grantees, strings.ToLower(privRole.Rolename))
			}
		}
	}
	return grantees
}

func extractDDLPermissions(stmt *pg_query.GrantStmt) (bool, []string) {
	// If Privileges is nil or empty, it represents ALL PRIVILEGES in PostgreSQL AST
	if len(stmt.Privileges) == 0 {
		return true, []string{"ALL PRIVILEGES"}
	}

	var ddlPerms []string
	isDDL := false

	for _, p := range stmt.Privileges {
		perm := ""
		if ap := p.GetAccessPriv(); ap != nil {
			perm = strings.ToUpper(ap.PrivName)
		} else if priv := p.GetString_(); priv != nil {
			perm = strings.ToUpper(priv.Sval)
		}

		switch perm {
		case "CREATE", "DROP", "TRUNCATE", "ALTER":
			isDDL = true
			ddlPerms = append(ddlPerms, perm)
		case "ALL", "":
			isDDL = true
			ddlPerms = append(ddlPerms, "ALL PRIVILEGES")
		}
	}

	return isDDL, ddlPerms
}
