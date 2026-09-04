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
				var targetViolation bool
				var isPublicTarget bool

				if grantee.IsPublic && reg.IsPublicForbidden() {
					targetViolation = true
					isPublicTarget = true
				} else if reg.IsAppRole(grantee.Name) {
					targetViolation = true
					isPublicTarget = false
				}

				if targetViolation {
					isDDL, ddlPerms := extractDDLPermissions(grantStmt)
					if isDDL {
						line := migration.FindLineForKeyword(content, grantee.Name)
						if dm != nil && (dm.IsLineIgnored(filename, line, RuleCode) || dm.IsLineIgnored(filename, line, RuleCode+".DDL-GRANT")) {
							continue
						}

						var msg string
						if isPublicTarget {
							msg = fmt.Sprintf("Forbidden grant of DDL permission (%s) to PUBLIC pseudo-role; cluster-wide DDL grants violate least-privilege isolation",
								strings.Join(ddlPerms, ", "))
						} else {
							msg = fmt.Sprintf("Forbidden grant of DDL permission (%s) to runtime role %q; app roles must be DML-only",
								strings.Join(ddlPerms, ", "), grantee.Name)
						}

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
					owner := resolveRoleSpec(cmd.Newowner)
					if owner == nil {
						continue
					}

					var isViolation bool
					var msg string
					if owner.IsPublic && reg.IsPublicForbidden() {
						isViolation = true
						msg = "Forbidden table ownership grant to PUBLIC pseudo-role; ownership must be retained by admin/migrator"
					} else if reg.IsAppRole(owner.Name) {
						isViolation = true
						msg = fmt.Sprintf("Forbidden table ownership grant to runtime app role %q; ownership must be retained by admin/migrator", owner.Name)
					}

					if isViolation {
						line := migration.FindLineForKeyword(content, owner.Name)
						if dm != nil && (dm.IsLineIgnored(filename, line, RuleCode) || dm.IsLineIgnored(filename, line, RuleCode+".OWNER-TO")) {
							continue
						}
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

		// 3. Inspect GRANT <role> TO <grantee> (administrative role membership)
		if grantRoleStmt := rawStmt.Stmt.GetGrantRoleStmt(); grantRoleStmt != nil && grantRoleStmt.IsGrant {
			for _, grNode := range grantRoleStmt.GranteeRoles {
				if roleSpec := grNode.GetRoleSpec(); roleSpec != nil {
					grantee := resolveRoleSpec(roleSpec)
					if grantee == nil {
						continue
					}
					if reg.IsAppRole(grantee.Name) || (grantee.IsPublic && reg.IsPublicForbidden()) {
						for _, gNode := range grantRoleStmt.GrantedRoles {
							if ap := gNode.GetAccessPriv(); ap != nil {
								if isAdministrativeRole(ap.PrivName) {
									line := migration.FindLineForKeyword(content, grantee.Name)
									if dm != nil && (dm.IsLineIgnored(filename, line, RuleCode) || dm.IsLineIgnored(filename, line, RuleCode+".ROLE-GRANT")) {
										continue
									}
									msg := fmt.Sprintf("Forbidden administrative role grant %q to runtime app role %q; app roles must not inherit administrative privileges",
										ap.PrivName, grantee.Name)
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
			}
		}
	}

	return issues
}

func extractGrantees(stmt *pg_query.GrantStmt) []GranteeInfo {
	var grantees []GranteeInfo
	for _, g := range stmt.Grantees {
		if privRole := g.GetRoleSpec(); privRole != nil {
			if info := resolveRoleSpec(privRole); info != nil {
				grantees = append(grantees, *info)
			}
		}
	}
	return grantees
}

func extractDDLPermissions(stmt *pg_query.GrantStmt) (bool, []string) {
	// In PostgreSQL grammar (gram.y), opt_privileges returning NIL (len == 0) signifies ALL [PRIVILEGES].
	// Whether ALL PRIVILEGES conveys DDL rights depends on the target object type:
	// - TABLE, SCHEMA, DATABASE: ALL conveys DDL (TRUNCATE, CREATE).
	// - SEQUENCE, FUNCTION, PROCEDURE, TYPE: ALL conveys DML/usage/execute only (zero DDL).
	if len(stmt.Privileges) == 0 {
		if isDDLApplicableObject(stmt.Objtype, stmt.Targtype) {
			return true, []string{"ALL PRIVILEGES"}
		}
		return false, nil
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
		case "ALL", "ALL PRIVILEGES", "":
			if isDDLApplicableObject(stmt.Objtype, stmt.Targtype) {
				isDDL = true
				ddlPerms = append(ddlPerms, "ALL PRIVILEGES")
			}
		}
	}

	return isDDL, ddlPerms
}

func isDDLApplicableObject(objtype pg_query.ObjectType, targtype pg_query.GrantTargetType) bool {
	if targtype == pg_query.GrantTargetType_ACL_TARGET_ALL_IN_SCHEMA {
		return objtype != pg_query.ObjectType_OBJECT_SEQUENCE &&
			objtype != pg_query.ObjectType_OBJECT_FUNCTION &&
			objtype != pg_query.ObjectType_OBJECT_PROCEDURE
	}

	switch objtype {
	case pg_query.ObjectType_OBJECT_TABLE,
		pg_query.ObjectType_OBJECT_SCHEMA,
		pg_query.ObjectType_OBJECT_DATABASE,
		pg_query.ObjectType_OBJECT_FDW,
		pg_query.ObjectType_OBJECT_FOREIGN_SERVER:
		return true
	case pg_query.ObjectType_OBJECT_SEQUENCE,
		pg_query.ObjectType_OBJECT_FUNCTION,
		pg_query.ObjectType_OBJECT_PROCEDURE,
		pg_query.ObjectType_OBJECT_ROUTINE,
		pg_query.ObjectType_OBJECT_TYPE,
		pg_query.ObjectType_OBJECT_DOMAIN:
		return false
	default:
		return true
	}
}
