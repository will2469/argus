package a15_ddl_grant

import (
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/will2469/argus/shared/config"
)

// DefaultAppRoles lists default runtime application roles guarded by ARGUS-A15.
// Note: PUBLIC is a PostgreSQL pseudo-role, governed separately via forbidPublicGrants.
var DefaultAppRoles = []string{"app_user", "web_app"}

// RoleRegistry tracks database roles designated as untrusted runtime application roles,
// as well as policy enforcement for the cluster-wide PostgreSQL PUBLIC pseudo-role.
type RoleRegistry struct {
	appRoles           map[string]bool
	forbidPublicGrants bool
}

// NewRoleRegistry constructs a registry with specified roles, defaulting if empty.
// If "public" is provided in the roles list, it configures forbidPublicGrants accordingly.
func NewRoleRegistry(roles []string) *RoleRegistry {
	forbidPublic := true
	if len(roles) == 0 {
		roles = DefaultAppRoles
	}

	rMap := make(map[string]bool, len(roles))
	for _, r := range roles {
		norm := strings.ToLower(strings.TrimSpace(r))
		if norm == "public" {
			forbidPublic = true
			continue
		}
		if norm != "" {
			rMap[norm] = true
		}
	}
	return &RoleRegistry{
		appRoles:           rMap,
		forbidPublicGrants: forbidPublic,
	}
}

// FromConfig constructs a RoleRegistry using configuration options for ARGUS-A15.
func FromConfig(cfg *config.Config) *RoleRegistry {
	if cfg == nil {
		return NewRoleRegistry(nil)
	}
	roles := cfg.GetStringSlice(RuleCode, "runtime_app_roles", DefaultAppRoles)
	reg := NewRoleRegistry(roles)
	reg.forbidPublicGrants = cfg.GetBool(RuleCode, "forbid_public_grants", true)
	return reg
}

// IsAppRole returns true if the specified role name is an untrusted runtime application role.
func (r *RoleRegistry) IsAppRole(roleName string) bool {
	if r == nil || len(r.appRoles) == 0 {
		return false
	}
	return r.appRoles[strings.ToLower(strings.TrimSpace(roleName))]
}

// IsPublicForbidden returns true if granting DDL permissions to the PUBLIC pseudo-role is forbidden.
func (r *RoleRegistry) IsPublicForbidden() bool {
	if r == nil {
		return true
	}
	return r.forbidPublicGrants
}

// GranteeInfo encapsulates normalized grantee identity and pseudo-role status.
type GranteeInfo struct {
	Name     string
	IsPublic bool
}

func resolveRoleSpec(spec *pg_query.RoleSpec) *GranteeInfo {
	if spec == nil {
		return nil
	}
	switch spec.Roletype {
	case pg_query.RoleSpecType_ROLESPEC_PUBLIC:
		return &GranteeInfo{Name: "public", IsPublic: true}
	case pg_query.RoleSpecType_ROLESPEC_CSTRING:
		name := strings.ToLower(strings.TrimSpace(spec.Rolename))
		return &GranteeInfo{Name: name, IsPublic: name == "public"}
	case pg_query.RoleSpecType_ROLESPEC_CURRENT_USER:
		return &GranteeInfo{Name: "current_user", IsPublic: false}
	case pg_query.RoleSpecType_ROLESPEC_CURRENT_ROLE:
		return &GranteeInfo{Name: "current_role", IsPublic: false}
	case pg_query.RoleSpecType_ROLESPEC_SESSION_USER:
		return &GranteeInfo{Name: "session_user", IsPublic: false}
	default:
		if spec.Rolename != "" {
			name := strings.ToLower(strings.TrimSpace(spec.Rolename))
			return &GranteeInfo{Name: name, IsPublic: name == "public"}
		}
	}
	return nil
}

var administrativeRoles = map[string]bool{
	"superuser":         true,
	"rds_superuser":     true,
	"cloudsqlsuperuser": true,
	"azure_pg_admin":    true,
	"pg_database_owner": true,
	"pg_read_all_data":  true,
	"pg_write_all_data": true,
	"admin":             true,
	"db_admin":          true,
}

func isAdministrativeRole(name string) bool {
	return administrativeRoles[strings.ToLower(strings.TrimSpace(name))]
}
