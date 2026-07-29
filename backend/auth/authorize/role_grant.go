package authorize

import (
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/models/auth"
)

// Which roles a caller may hand out when they create school access for someone
// else (staff invitations, account registration, link-to-tenant). The decision
// lives here, next to the other role decisions, so every entry point shares one
// rule instead of re-deciding it per handler (Rule 9, Rule 12).
//
// The escalation this prevents: the "user" (Betreuer) system role carries
// users:create globally — migration 1.5.3 grants it so Betreuer can create
// person records. Without this check, holding users:create was enough to invite
// someone into the admin role and then log in as that account.

// EffectiveBaseRole returns the privilege tier a role hands out, as one of the
// values in auth.ValidBaseRoles(). Custom roles carry base_role explicitly (the
// create API requires it); system roles predate the column and are identified by
// name. Returns "" when the tier cannot be determined; callers must fail
// closed unless they can prove the target permissions do not exceed the
// caller's permissions for a legacy tenant role.
func EffectiveBaseRole(role *auth.Role) string {
	if role == nil {
		return ""
	}

	if role.BaseRole != nil {
		if base := strings.ToLower(strings.TrimSpace(*role.BaseRole)); base != "" {
			return base
		}
	}

	// System roles (tenant_id IS NULL) were seeded before base_role existed and
	// were never backfilled, so fall back to the name. Legacy system roles whose
	// name is not a base role ("guest", "teacher", "staff") stay unknown on
	// purpose — they are not roles anyone should be handing out today.
	if role.IsSystem {
		name := strings.ToLower(strings.TrimSpace(role.Name))
		switch name {
		case auth.BaseRoleAdmin, auth.BaseRoleUser, auth.BaseRoleGuardian:
			return name
		}
	}

	return ""
}

// CanGrantRole reports whether a caller holding callerPermissions may assign the
// given role to another account.
//
// Granting an admin-tier role requires users:manage. User- and guardian-tier
// roles may only be handed out when every effective permission of the target
// role is already held by the caller. base_role is a classification for UI and
// portal behavior, not a permission boundary, so a tenant role with
// base_role=user is not implicitly safe. Legacy tenant roles without a
// base_role use the same effective-permission check; unknown system roles
// remain fail-closed.
// Whether a guardian role may be handed out through a staff flow at all is a
// separate question, answered by ValidateAssignableSchoolRole.
func CanGrantRole(role *auth.Role, callerPermissions []string) bool {
	if role == nil {
		return false
	}

	if HasPermission(permissions.UsersManage, callerPermissions) {
		return true
	}

	switch EffectiveBaseRole(role) {
	case auth.BaseRoleUser, auth.BaseRoleGuardian:
		return permissionsAreSubset(role.Permissions, callerPermissions)
	case "":
		if role.TenantID != nil && role.BaseRole == nil {
			return permissionsAreSubset(role.Permissions, callerPermissions)
		}
		return false
	default:
		return false
	}
}

func permissionsAreSubset(rolePermissions []*auth.Permission, callerPermissions []string) bool {
	for _, permission := range rolePermissions {
		if permission == nil || !HasPermission(permission.Name, callerPermissions) {
			return false
		}
	}
	return true
}
