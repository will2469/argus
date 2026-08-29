package a15_ddl_grant

import (
	"strings"

	"github.com/will2469/argus/shared/config"
)

// DefaultAppRoles lists default runtime application roles guarded by ARGUS-A15.
var DefaultAppRoles = []string{"app_user", "web_app", "public"}

// RoleRegistry tracks database roles designated as untrusted runtime application roles.
type RoleRegistry struct {
	roles map[string]bool
}

// NewRoleRegistry constructs a registry with specified roles, defaulting if empty.
func NewRoleRegistry(roles []string) *RoleRegistry {
	if len(roles) == 0 {
		roles = DefaultAppRoles
	}
	rMap := make(map[string]bool, len(roles))
	for _, r := range roles {
		rMap[strings.ToLower(strings.TrimSpace(r))] = true
	}
	return &RoleRegistry{roles: rMap}
}

// FromConfig constructs a RoleRegistry using configuration options for ARGUS-A15.
func FromConfig(cfg *config.Config) *RoleRegistry {
	if cfg == nil {
		return NewRoleRegistry(nil)
	}
	roles := cfg.GetStringSlice(RuleCode, "runtime_app_roles", DefaultAppRoles)
	return NewRoleRegistry(roles)
}

// IsAppRole returns true if the specified role name is an untrusted runtime application role.
func (r *RoleRegistry) IsAppRole(roleName string) bool {
	if r == nil || len(r.roles) == 0 {
		return false
	}
	return r.roles[strings.ToLower(strings.TrimSpace(roleName))]
}
