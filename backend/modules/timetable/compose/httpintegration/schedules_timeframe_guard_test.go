package httpintegration_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	schedulesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchedulesTimeframeGuardRefusesChangesACareOfferingNeeds: the schedules
// API runs every timeframe edit and deletion through the care-offering guard
// first. A refusal answers 409 with the stable wording and leaves the row
// untouched; the guard sees the proposed row for an edit and nothing for a
// deletion.
func TestSchedulesTimeframeGuardRefusesChangesACareOfferingNeeds(t *testing.T) {
	t.Parallel()
	db, services := testutil.SetupScheduleModule(t)
	timeframe := testpkg.CreateTestTimeframeForTenant(t, db, testpkg.Tenant(t), "guarded")

	var seen []*timetableModule.TimeframeInput
	guard := schedulesAPI.TimeframeChangeGuard(func(_ context.Context, id int64, replacement *timetableModule.TimeframeInput) error {
		assert.Equal(t, timeframe.ID, id)
		seen = append(seen, replacement)
		return fmt.Errorf("%w: no complete timeframe", timetableModule.ErrTimeframeRequiredByCareOffering)
	})
	resource := schedulesAPI.NewSchedulesResource(services.Calendar, services.Timetable, guard, db)
	router := resource.Router()

	update := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/timeframes/%d", timeframe.ID), map[string]any{
		"start_time": "2026-01-14T09:00:00Z", "is_active": true, "description": "offen",
	})
	response := testutil.ExecuteWithAuth(t, router, update, testutil.AdminTestClaims(1))
	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), "geändert oder gelöscht")
	require.Len(t, seen, 1)
	require.NotNil(t, seen[0], "an edit hands the guard the proposed row")
	assert.Nil(t, seen[0].EndTime, "the proposed row is the open-ended replacement")

	stored, err := services.Timetable.FindTimeframe(testpkg.Ctx(t), timeframe.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.EndTime, "a refused edit leaves the timeframe unchanged")

	remove := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/timeframes/%d", timeframe.ID), nil)
	response = testutil.ExecuteWithAuth(t, router, remove, testutil.AdminTestClaims(1))
	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	require.Len(t, seen, 2)
	assert.Nil(t, seen[1], "a deletion hands the guard no replacement")
	_, err = services.Timetable.FindTimeframe(testpkg.Ctx(t), timeframe.ID)
	require.NoError(t, err, "a refused deletion leaves the timeframe intact")
}
