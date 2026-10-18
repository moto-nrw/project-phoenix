package securityruntime

// The configuration permissions restate the permission registry's names,
// which this public package may not import; a test pins them to it.
const (
	permissionConfigRead   = "config:read"
	permissionConfigUpdate = "config:update"
	permissionUsersRead    = "users:read"
)

// AccessRule is the permission check of a route: any one of Permissions
// grants access, or all of them when RequireAll is set.
type AccessRule struct {
	Permissions []string
	RequireAll  bool
}

// ConfigAccess is an access level of the settings routes.
type ConfigAccess uint8

const (
	// ConfigAccessWrite changes settings. It is also the level of an
	// unknown value, so an unmapped level never opens more than a write.
	ConfigAccessWrite ConfigAccess = iota
	ConfigAccessRead
	ConfigAccessManage
	ConfigAccessReadOrWrite
)

// ConfigAccessPermissions names the permissions of a settings access level:
// reading takes config:read, managing config:manage, the combined level any
// configuration permission, and writing (the default) config:update or
// config:manage.
func ConfigAccessPermissions(level ConfigAccess) AccessRule {
	switch level {
	case ConfigAccessRead:
		return AccessRule{Permissions: []string{permissionConfigRead}}
	case ConfigAccessManage:
		return AccessRule{Permissions: []string{permissionConfigManage}}
	case ConfigAccessReadOrWrite:
		return AccessRule{Permissions: []string{permissionConfigRead, permissionConfigUpdate, permissionConfigManage}}
	default:
		return AccessRule{Permissions: configWritePermissions()}
	}
}

func configWritePermissions() []string {
	return []string{permissionConfigUpdate, permissionConfigManage}
}

// CanEditConfig reports whether a caller holding the permissions hasPermission
// answers for may change settings.
func CanEditConfig(hasPermission func(string) bool) bool {
	for _, permission := range configWritePermissions() {
		if hasPermission(permission) {
			return true
		}
	}
	return false
}

// MealPlanAccess is an access level of the meal plan routes.
type MealPlanAccess uint8

const (
	// MealPlanAccessWrite changes the meal plan. It is also the level of an
	// unknown value.
	MealPlanAccessWrite MealPlanAccess = iota
	MealPlanAccessRead
	MealPlanAccessParticipants
)

// MealPlanAccessPermissions names the permissions of a meal plan access
// level: reading takes config:read, the kitchen list both config:read and
// users:read (it combines the meal settings with student directory data),
// and writing (the default) config:update.
func MealPlanAccessPermissions(level MealPlanAccess) AccessRule {
	switch level {
	case MealPlanAccessParticipants:
		return AccessRule{Permissions: MealParticipantsPermissions(), RequireAll: true}
	case MealPlanAccessRead:
		return AccessRule{Permissions: []string{permissionConfigRead}}
	default:
		return AccessRule{Permissions: []string{permissionConfigUpdate}}
	}
}

// MealParticipantsPermissions are the permissions the kitchen list needs,
// all of them.
func MealParticipantsPermissions() []string {
	return []string{permissionConfigRead, permissionUsersRead}
}
