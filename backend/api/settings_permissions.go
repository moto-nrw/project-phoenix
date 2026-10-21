package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/api/common"
	mealplanAPI "github.com/moto-nrw/project-phoenix/modules/mealplan/http"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/http"
)

// The settings and meal plan routes take their permission sets from Security
// Runtime (#4082); the maps below only translate each HTTP package's access
// level into Security Runtime's. An unmapped level reads as Security
// Runtime's zero level, the write.

var settingsConfigAccess = map[settingsHTTP.Access]securityruntime.ConfigAccess{
	settingsHTTP.AccessRead:        securityruntime.ConfigAccessRead,
	settingsHTTP.AccessWrite:       securityruntime.ConfigAccessWrite,
	settingsHTTP.AccessManage:      securityruntime.ConfigAccessManage,
	settingsHTTP.AccessReadOrWrite: securityruntime.ConfigAccessReadOrWrite,
}

var mealPlanAccess = map[mealplanAPI.Access]securityruntime.MealPlanAccess{
	mealplanAPI.AccessRead:              securityruntime.MealPlanAccessRead,
	mealplanAPI.AccessWrite:             securityruntime.MealPlanAccessWrite,
	mealplanAPI.AccessParticipants:      securityruntime.MealPlanAccessParticipants,
	mealplanAPI.AccessRegister:          securityruntime.MealPlanAccessRegister,
	mealplanAPI.AccessDailyParticipants: securityruntime.MealPlanAccessDailyParticipants,
}

// accessGuard builds the route guard of a Security Runtime access rule.
func accessGuard(rule securityruntime.AccessRule) common.Middleware {
	if rule.RequireAll {
		return common.RequiresAllPermissions(rule.Permissions...)
	}
	return common.RequiresAnyPermission(rule.Permissions...)
}

func requireSettingsAccess(access settingsHTTP.Access) common.Middleware {
	return accessGuard(securityruntime.ConfigAccessPermissions(settingsConfigAccess[access]))
}

func requireMealPlanAccess(access mealplanAPI.Access) common.Middleware {
	return accessGuard(securityruntime.MealPlanAccessPermissions(mealPlanAccess[access]))
}

// mealPlanAllowed reports whether the caller passes a meal plan access level.
func mealPlanAllowed(ctx context.Context, access mealplanAPI.Access) bool {
	principal, err := common.CurrentPrincipal(ctx)
	return err == nil && securityruntime.MealPlanAccessPermissions(mealPlanAccess[access]).AllowedBy(principal.HasPermission)
}

// canEditConfig reports whether the caller may change settings.
func canEditConfig(ctx context.Context) bool {
	principal, err := common.CurrentPrincipal(ctx)
	return err == nil && securityruntime.CanEditConfig(principal.HasPermission)
}
