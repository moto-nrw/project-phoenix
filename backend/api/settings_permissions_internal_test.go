package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"

	mealplanAPI "github.com/moto-ogs/project-phoenix/modules/mealplan/http"
	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	settingsHTTP "github.com/moto-ogs/project-phoenix/modules/settings/http"
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
		mealplanAPI.AccessBillingRelease:    securityruntime.MealPlanAccessBillingRelease,
	}, mealPlanAccess)
}

// #4686: the Mensa-Monatsfreigabe shows the children's names and closes a
// month, so it needs both the names and the maintenance of the Mensa.
func TestMealBillingReleaseNeedsTheNamesAndTheMensa(t *testing.T) {
	t.Parallel()
	caller := func(granted ...string) context.Context {
		principal, err := permissions.NewPrincipal(permissions.PrincipalInput{AccountID: 98, TenantID: 22, Scope: string(permissions.ScopeTenant), Permissions: granted})
		require.NoError(t, err)
		return permissions.WithPrincipal(context.Background(), principal)
	}
	assert.False(t, mealPlanAllowed(caller(permissions.MealsOrders), mealplanAPI.AccessBillingRelease), "names alone release nothing")
	assert.False(t, mealPlanAllowed(caller(permissions.MealsManage), mealplanAPI.AccessBillingRelease), "the Mensa alone shows no names")
	assert.True(t, mealPlanAllowed(caller(permissions.MealsManage, permissions.MealsOrders), mealplanAPI.AccessBillingRelease))
	assert.True(t, mealPlanAllowed(caller("admin:*"), mealplanAPI.AccessBillingRelease))
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
