package securityruntime_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// The sets pin the guards the root composed before Security Runtime named
// them: RequiresPermission(config:read), RequiresPermission(config:manage),
// RequiresAnyPermission(config:read, config:update, config:manage) and, as the
// default, RequiresAnyPermission(config:update, config:manage) for the
// settings; RequiresAllPermissions(config:read, users:read),
// RequiresPermission(config:read) and, as the default,
// RequiresPermission(config:update) for the meal plan.
func TestConfigAccessPermissionsKeepTheComposedGuards(t *testing.T) {
	t.Parallel()
	write := securityruntime.AccessRule{Permissions: []string{permissions.ConfigUpdate, permissions.ConfigManage}}
	assert.Equal(t, securityruntime.AccessRule{Permissions: []string{permissions.ConfigRead}}, securityruntime.ConfigAccessPermissions(securityruntime.ConfigAccessRead))
	assert.Equal(t, securityruntime.AccessRule{Permissions: []string{permissions.ConfigManage}}, securityruntime.ConfigAccessPermissions(securityruntime.ConfigAccessManage))
	assert.Equal(t, securityruntime.AccessRule{Permissions: []string{permissions.ConfigRead, permissions.ConfigUpdate, permissions.ConfigManage}}, securityruntime.ConfigAccessPermissions(securityruntime.ConfigAccessReadOrWrite))
	assert.Equal(t, write, securityruntime.ConfigAccessPermissions(securityruntime.ConfigAccessWrite))
	assert.Equal(t, write, securityruntime.ConfigAccessPermissions(securityruntime.ConfigAccess(200)), "an unknown level is a write")

	assert.Equal(t, securityruntime.AccessRule{Permissions: []string{permissions.ConfigRead, permissions.UsersRead}, RequireAll: true}, securityruntime.MealPlanAccessPermissions(securityruntime.MealPlanAccessParticipants))
	assert.Equal(t, securityruntime.AccessRule{Permissions: []string{permissions.ConfigRead}}, securityruntime.MealPlanAccessPermissions(securityruntime.MealPlanAccessRead))
	assert.Equal(t, securityruntime.AccessRule{Permissions: []string{permissions.ConfigUpdate}}, securityruntime.MealPlanAccessPermissions(securityruntime.MealPlanAccessWrite))
	assert.Equal(t, securityruntime.AccessRule{Permissions: []string{permissions.ConfigUpdate}}, securityruntime.MealPlanAccessPermissions(securityruntime.MealPlanAccess(200)), "an unknown level is a write")
	assert.Equal(t, []string{permissions.ConfigRead, permissions.UsersRead}, securityruntime.MealParticipantsPermissions())
}

func TestCanEditConfigNeedsUpdateOrManage(t *testing.T) {
	t.Parallel()
	holding := func(granted ...string) func(string) bool {
		return func(permission string) bool { return slices.Contains(granted, permission) }
	}
	assert.True(t, securityruntime.CanEditConfig(holding(permissions.ConfigUpdate)))
	assert.True(t, securityruntime.CanEditConfig(holding(permissions.ConfigManage)))
	assert.False(t, securityruntime.CanEditConfig(holding(permissions.ConfigRead)))
}
