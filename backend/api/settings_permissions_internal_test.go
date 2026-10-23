package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
		mealplanAPI.AccessParticipantsWrite: securityruntime.MealPlanAccessParticipantsWrite,
	}, mealPlanAccess)
}
