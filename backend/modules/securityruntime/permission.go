package securityruntime

import "github.com/moto-nrw/project-phoenix/auth/authorize"

// HasPermission evaluates a required permission with the shared wildcard rules.
func HasPermission(required string, permissions []string) bool {
	return authorize.HasPermission(required, permissions)
}

// HasAdminWildcard reports a system-wide admin permission (admin:* or *:*).
func HasAdminWildcard(permissions []string) bool {
	return authorize.HasAdminWildcard(permissions)
}

// DatabaseStatsCapabilities evaluates which database statistics the
// permissions reveal.
func DatabaseStatsCapabilities(permissions []string) authorize.DatabaseStatsCapabilities {
	return authorize.NewDatabaseStatsCapabilities(permissions)
}

// DatabaseStatsPermissions lists every permission the statistics honour.
func DatabaseStatsPermissions() []string {
	return authorize.DatabaseStatsPermissions()
}

// PermissionCalendarOwn is the permission a staff member needs to use the
// calendar for themselves. It restates the permission registry's name, which
// this public package may not import; a test pins it to it.
const PermissionCalendarOwn = "calendar:own"

// PermissionStaffManage gates writes to another person's general staff record;
// creating a staff member over a person that already carries one adopts the
// record and owes this permission (#2906). PermissionGroupsRead is the
// permission a new staff member's account is granted so the colleague sees the
// group list. Both restate the permission registry's names, which this public
// package may not import; a test pins them to it.
const (
	PermissionStaffManage = "staff:manage"
	PermissionGroupsRead  = "groups:read"
)
