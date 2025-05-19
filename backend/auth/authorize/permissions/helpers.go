package permissions

import (
	"strings"
)

// GetAllResourcePermissions returns all defined permissions for a specific resource.
// This is useful for granting full access to a resource type.
func GetAllResourcePermissions(resource string) []string {
	var perms []string

	// Standard action permissions
	perms = append(perms,
		resource+":"+ActionCreate,
		resource+":"+ActionRead,
		resource+":"+ActionUpdate,
		resource+":"+ActionDelete,
		resource+":"+ActionList,
		resource+":"+ActionManage,
	)

	// Add special actions based on resource type
	switch resource {
	case ResourceActivities:
		perms = append(perms,
			ResourceActivities+":enroll",
			ResourceActivities+":assign",
		)
	case ResourceGroups:
		perms = append(perms,
			ResourceGroups+":assign",
		)
	}

	return perms
}

// GetAdminPermissions returns all permissions needed for an administrator.
func GetAdminPermissions() []string {
	return []string{AdminWildcard}
}

// IsAdminPermission checks if the given permission is an admin-level permission.
func IsAdminPermission(permission string) bool {
	return permission == AdminWildcard || permission == FullAccess
}

// IsResourcePermission checks if the permission is for a specific resource.
func IsResourcePermission(permission, resource string) bool {
	parts := strings.Split(permission, ":")
	if len(parts) != 2 {
		return false
	}

	return parts[0] == resource || parts[0] == "*"
}

// IsActionPermission checks if the permission grants a specific action.
func IsActionPermission(permission, action string) bool {
	parts := strings.Split(permission, ":")
	if len(parts) != 2 {
		return false
	}

	return parts[1] == action || parts[1] == "*"
}

// GetStandardRolePermissions returns a map of predefined role names to their permissions.
// This can be used to initialize standard roles in the system.
func GetStandardRolePermissions() map[string][]string {
	return map[string][]string{
		"admin": {AdminWildcard},

		"user_manager": {
			UsersCreate, UsersRead, UsersUpdate, UsersDelete, UsersList, UsersManage,
		},

		"activity_manager": {
			ActivitiesCreate, ActivitiesRead, ActivitiesUpdate, ActivitiesDelete, ActivitiesList, ActivitiesManage,
			ActivitiesEnroll, ActivitiesAssign,
		},

		"room_manager": {
			RoomsCreate, RoomsRead, RoomsUpdate, RoomsDelete, RoomsList, RoomsManage,
		},

		"group_manager": {
			GroupsCreate, GroupsRead, GroupsUpdate, GroupsDelete, GroupsList, GroupsManage,
			GroupsAssign,
		},

		"feedback_manager": {
			FeedbackCreate, FeedbackRead, FeedbackDelete, FeedbackList, FeedbackManage,
		},

		"config_manager": {
			ConfigRead, ConfigUpdate, ConfigManage,
		},

		"iot_manager": {
			IOTRead, IOTUpdate, IOTManage,
		},

		// Read-only roles
		"user_viewer": {
			UsersRead, UsersList,
		},

		"activity_viewer": {
			ActivitiesRead, ActivitiesList,
		},

		"room_viewer": {
			RoomsRead, RoomsList,
		},

		"group_viewer": {
			GroupsRead, GroupsList,
		},

		"feedback_viewer": {
			FeedbackRead, FeedbackList,
		},
	}
}

// HasPermissionForResource checks if the list of permissions contains any permission
// that grants access to the specified resource.
func HasPermissionForResource(permissions []string, resource string) bool {
	for _, perm := range permissions {
		if IsResourcePermission(perm, resource) {
			return true
		}

		// Admin wildcards always grant access
		if IsAdminPermission(perm) {
			return true
		}
	}

	return false
}

// FilterPermissionsByResource returns only permissions related to a specific resource.
func FilterPermissionsByResource(permissions []string, resource string) []string {
	var result []string

	for _, perm := range permissions {
		if IsResourcePermission(perm, resource) {
			result = append(result, perm)
		}

		// Include admin wildcards
		if IsAdminPermission(perm) {
			result = append(result, perm)
		}
	}

	return result
}
