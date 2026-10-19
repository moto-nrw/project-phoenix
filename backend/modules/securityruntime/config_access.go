package securityruntime

// The configuration permissions restate the permission registry's names,
// which this public package may not import; a test pins them to it.
const (
	permissionConfigRead    = "config:read"
	permissionConfigUpdate  = "config:update"
	permissionUsersRead     = "users:read"
	permissionMealsRead     = "meals:read"
	permissionMealsManage   = "meals:manage"
	permissionMealsOrders   = "meals:orders"
	permissionMealsRegister = "meals:register"
)

// AccessRule is the permission check of a route: any one of Permissions
// grants access, or all of them when RequireAll is set.
type AccessRule struct {
	Permissions []string
	RequireAll  bool
}

// AllowedBy reports whether a caller holding the permissions hasPermission
// answers for passes the rule.
func (rule AccessRule) AllowedBy(hasPermission func(string) bool) bool {
	if rule.RequireAll {
		for _, permission := range rule.Permissions {
			if !hasPermission(permission) {
				return false
			}
		}
		return len(rule.Permissions) > 0
	}
	for _, permission := range rule.Permissions {
		if hasPermission(permission) {
			return true
		}
	}
	return false
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
	// MealPlanAccessRegister changes a child's registration for the school.
	MealPlanAccessRegister
)

// MealPlanAccessPermissions names the permission of a meal plan access level
// (#3873): reading takes meals:read, the lists with children's names
// meals:orders, a child's registration changed by the school meals:register,
// and writing (the default) meals:manage. A kitchen role no longer needs the
// configuration or every child's data, and seeing names opens no change.
func MealPlanAccessPermissions(level MealPlanAccess) AccessRule {
	switch level {
	case MealPlanAccessRegister:
		return AccessRule{Permissions: []string{permissionMealsRegister}}
	case MealPlanAccessParticipants:
		return AccessRule{Permissions: MealParticipantsPermissions()}
	case MealPlanAccessRead:
		return AccessRule{Permissions: []string{permissionMealsRead}}
	default:
		return AccessRule{Permissions: []string{permissionMealsManage}}
	}
}

// MealParticipantsPermissions are the permissions the lists with children's
// names need.
func MealParticipantsPermissions() []string {
	return []string{permissionMealsOrders}
}
