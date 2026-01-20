package tenant

// RolePermissions maps BetterAuth role names to their permissions.
//
// CRITICAL: This map MUST stay in sync with betterauth/src/permissions.ts!
// If you modify role permissions in BetterAuth, update this map immediately.
//
// Permission format: "resource:action"
// Resources: student, group, room, attendance, location, staff, ogs
// Actions vary by resource (read, create, update, delete, etc.)
//
// GDPR NOTE: The "location:read" permission is GDPR-sensitive.
// Only operational roles (supervisor, ogsAdmin) who work directly with students
// have this permission. Administrative roles (bueroAdmin, traegerAdmin) who
// manage remotely do NOT have location access - this is a legal requirement.
var RolePermissions = map[string][]string{
	// supervisor - Front-line staff working directly with students.
	// Scope: Only their assigned groups (enforced at query level, not here).
	// CAN see location data (operational need).
	"supervisor": {
		"student:read", "student:update",
		"group:read",
		"attendance:read", "attendance:checkin", "attendance:checkout",
		"location:read", // Operational staff needs to see where students are
	},

	// ogsAdmin - Full administrator for a single OGS (after-school center).
	// Scope: All data within their OGS.
	// CAN see location data (runs the OGS operationally).
	"ogsAdmin": {
		"student:read", "student:create", "student:update", "student:delete",
		"group:read", "group:create", "group:update", "group:delete", "group:assign",
		"room:read", "room:create", "room:update", "room:delete",
		"attendance:read", "attendance:checkin", "attendance:checkout",
		"location:read", // OGS admin runs operations, needs location visibility
		"staff:read", "staff:create", "staff:update", "staff:invite",
		"ogs:read", "ogs:update",
	},

	// bueroAdmin - Office administrator managing multiple OGS remotely.
	// Scope: All OGS under their Büro (office).
	// GDPR: NO location:read permission - manages from distance.
	"bueroAdmin": {
		"student:read", "student:create", "student:update", "student:delete",
		"group:read", "group:create", "group:update", "group:delete",
		"attendance:read",
		// location: INTENTIONALLY OMITTED - GDPR compliance!
		"staff:read", "staff:create", "staff:update", "staff:delete", "staff:invite",
		"ogs:read", "ogs:update",
	},

	// traegerAdmin - Carrier administrator (highest level), manages all OGS.
	// Scope: All OGS under their Träger (carrier/provider).
	// GDPR: NO location:read permission - highest admin level, manages remotely.
	"traegerAdmin": {
		"student:read", "student:create", "student:update", "student:delete",
		"group:read", "group:create", "group:update", "group:delete",
		"attendance:read",
		// location: INTENTIONALLY OMITTED - GDPR compliance!
		"staff:read", "staff:create", "staff:update", "staff:delete", "staff:invite",
		"ogs:read", "ogs:update",
	},
}

// GetPermissionsForRole returns the permissions for a given role.
// Returns an empty slice if the role is unknown.
func GetPermissionsForRole(role string) []string {
	perms, ok := RolePermissions[role]
	if !ok {
		return []string{}
	}
	// Return a copy to prevent modification of the original
	result := make([]string, len(perms))
	copy(result, perms)
	return result
}

// IsValidRole checks if a role name is recognized.
func IsValidRole(role string) bool {
	_, ok := RolePermissions[role]
	return ok
}

// AllRoles returns all valid role names.
func AllRoles() []string {
	return []string{"supervisor", "ogsAdmin", "bueroAdmin", "traegerAdmin"}
}

// RolesWithPermission returns all roles that have a specific permission.
// Useful for documentation and debugging.
func RolesWithPermission(permission string) []string {
	var roles []string
	for role, perms := range RolePermissions {
		for _, p := range perms {
			if p == permission {
				roles = append(roles, role)
				break
			}
		}
	}
	return roles
}
