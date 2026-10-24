package securityruntime

import (
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime/internal/application/authorize"
)

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
func DatabaseStatsCapabilities(permissions []string) interface {
	ViewStudents() bool
	ViewTeachers() bool
	ViewRooms() bool
	ViewActivities() bool
	ViewGroups() bool
	ViewRoles() bool
	ViewDevices() bool
	ViewPermissionCatalog() bool
	ViewTimetables() bool
	ViewGradeTransitions() bool
} {
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
// record and owes this permission (#2906). It restates the permission
// registry's name, which this public package may not import; a test pins it.
const PermissionStaffManage = "staff:manage"

// permissionConfigManage is the permission a school's own lead role carries
// next to the admin scope. It restates the permission registry's name.
const permissionConfigManage = "config:manage"

// LeadsSchool reports whether a role makes its holder the school's lead,
// as the staff frontend's leadsSchool reads it: the system admin role, or a
// role the school created with config:manage.
func LeadsSchool(roleName string, rolePermissions []string) bool {
	return strings.EqualFold(strings.TrimSpace(roleName), "admin") ||
		HasPermission(permissionConfigManage, rolePermissions)
}
