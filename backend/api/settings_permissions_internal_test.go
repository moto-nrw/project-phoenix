package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"

	mealplanAPI "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/http"
)

func TestAccessLevelsMapOntoSecurityRuntimeLevels(t *testing.T) {
	t.Parallel()
	assert.Equal(t, map[settingsHTTP.Access]securityruntime.ConfigAccess{
		settingsHTTP.AccessRead:        securityruntime.ConfigAccessRead,
		settingsHTTP.AccessWrite:       securityruntime.ConfigAccessWrite,
		settingsHTTP.AccessManage:      securityruntime.ConfigAccessManage,
		settingsHTTP.AccessReadOrWrite: securityruntime.ConfigAccessReadOrWrite,
	}, settingsConfigAccess)
	assert.Equal(t, map[mealplanAPI.Access]securityruntime.MealPlanAccess{
		mealplanAPI.AccessRead:              securityruntime.MealPlanAccessRead,
		mealplanAPI.AccessWrite:             securityruntime.MealPlanAccessWrite,
		mealplanAPI.AccessParticipants:      securityruntime.MealPlanAccessParticipants,
		mealplanAPI.AccessRegister:          securityruntime.MealPlanAccessRegister,
		mealplanAPI.AccessDailyParticipants: securityruntime.MealPlanAccessDailyParticipants,
	}, mealPlanAccess)
}

// #3873: seeing the lists with names opens no change of a registration.
func TestMealPlanAllowedFollowsTheRegisterPermission(t *testing.T) {
	t.Parallel()
	const accountID, tenantID = 99, 23
	caller := func(granted ...string) context.Context {
		principal, err := permissions.NewPrincipal(permissions.PrincipalInput{AccountID: accountID, TenantID: tenantID, Scope: string(permissions.ScopeTenant), Permissions: granted})
		require.NoError(t, err)
		return permissions.WithPrincipal(context.Background(), principal)
	}
	assert.False(t, mealPlanAllowed(caller(permissions.MealsOrders), mealplanAPI.AccessRegister))
	assert.True(t, mealPlanAllowed(caller(permissions.MealsOrders), mealplanAPI.AccessParticipants))
	assert.True(t, mealPlanAllowed(caller(permissions.MealsRegister), mealplanAPI.AccessRegister))
	assert.True(t, mealPlanAllowed(caller(permissions.MealsRegister), mealplanAPI.AccessDailyParticipants))
	assert.True(t, mealPlanAllowed(caller(permissions.MealsOrders), mealplanAPI.AccessDailyParticipants))
	assert.False(t, mealPlanAllowed(caller(permissions.MealsRead), mealplanAPI.AccessDailyParticipants))
	assert.False(t, mealPlanAllowed(caller(permissions.MealsRegister), mealplanAPI.AccessParticipants), "registration alone does not grant exports")
	assert.True(t, mealPlanAllowed(caller("admin:*"), mealplanAPI.AccessRegister), "admin:* holds every permission, as at the route guard")
	assert.False(t, mealPlanAllowed(context.Background(), mealplanAPI.AccessRead), "no caller, no access")
}
