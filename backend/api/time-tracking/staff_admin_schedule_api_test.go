package timetracking

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestUpdateSchedule_SaveAsTemplateMaterializesAssignedSnapshot(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "ScheduleTemplate", "Clean")
	repos := repositories.NewFactory(ctx.db)
	repos.SetConfigRuntime(testpkg.ConfigRuntime(ctx.db))

	require.NoError(t, repos.StaffWorkSchedule.ReplaceSchedule(testpkg.Ctx(t), staff.ID, []*configModels.StaffWorkSchedule{
		{
			WeekIndex:      0,
			RotationLength: 1,
			DayOfWeek:      configModels.DayMonday,
			TargetMinutes:  300,
		},
	}, configModels.CalendarDate("")))

	claims := testutil.DefaultTestClaims()
	claims.Permissions = []string{"time_tracking:manage"}
	token := testutil.MintTestJWT(t, claims)
	body := map[string]any{
		"mode":                 "custom",
		"rotation_length":      1,
		"rotation_anchor_date": "2026-06-01",
		"save_as_template":     fmt.Sprintf("Saved schedule template clean %d", staff.ID),
		"entries": []map[string]any{
			{
				"week_index":     0,
				"day_of_week":    configModels.DayTuesday,
				"target_minutes": 360,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/staff/%d/schedule", staff.ID), body, testutil.WithJWTBearer(token))
	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	activeRows, err := repos.StaffWorkSchedule.GetCurrentByStaffID(testpkg.Ctx(t), staff.ID)
	require.NoError(t, err)
	require.Len(t, activeRows, 1)
	assert.Equal(t, configModels.DayTuesday, activeRows[0].DayOfWeek)
	assert.Equal(t, 360, activeRows[0].TargetMinutes)

	reloadedStaff, err := repositories.NewFactory(ctx.db).Staff.FindByID(testpkg.Ctx(t), staff.ID)
	require.NoError(t, err)
	require.NotNil(t, reloadedStaff.WorkTimeModelID)
	// config.work_time_model_entries has no tenant of its own; it only goes
	// away with its model, so the delete has to actually happen (#2419).
	t.Cleanup(func() {
		_, err := ctx.db.NewUpdate().TableExpr("users.staff").
			Set("work_time_model_id = NULL").
			Where("work_time_model_id = ?", *reloadedStaff.WorkTimeModelID).
			Exec(context.Background())
		require.NoError(t, err)
		_, err = ctx.db.NewDelete().TableExpr("config.work_time_models").
			Where("id = ?", *reloadedStaff.WorkTimeModelID).
			Exec(context.Background())
		require.NoError(t, err)
	})

	model, err := repos.WorkTimeModel.FindByID(testpkg.Ctx(t), *reloadedStaff.WorkTimeModelID)
	require.NoError(t, err)
	require.Len(t, model.Entries, 1)
	assert.Equal(t, configModels.DayTuesday, model.Entries[0].DayOfWeek)
	assert.Equal(t, 360, model.Entries[0].TargetMinutes)
}

func TestGetSchedule_AllowsOwnStaffWithTimeTrackingOwn(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "ScheduleOwn", "Read")
	repos := repositories.NewFactory(ctx.db)
	repos.SetConfigRuntime(testpkg.ConfigRuntime(ctx.db))

	require.NoError(t, repos.StaffWorkSchedule.ReplaceSchedule(testpkg.Ctx(t), staff.ID, []*configModels.StaffWorkSchedule{
		{
			WeekIndex:      0,
			RotationLength: 1,
			DayOfWeek:      configModels.DayMonday,
			TargetMinutes:  300,
		},
	}, configModels.CalendarDate("")))

	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.Permissions = []string{"time_tracking:own"}
	token := testutil.MintTestJWT(t, claims)

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/%d/schedule", staff.ID), nil, testutil.WithJWTBearer(token))
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetSchedule_RejectsOtherStaffWithTimeTrackingOwn(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "ScheduleOwn", "Only")
	otherStaff := testpkg.CreateTestStaff(t, ctx.db, "ScheduleOther", "Denied")

	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.Permissions = []string{"time_tracking:own"}
	token := testutil.MintTestJWT(t, claims)

	req := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/staff/%d/schedule", otherStaff.ID), nil, testutil.WithJWTBearer(token))
	rr := testutil.ExecuteRequest(ctx.router, req)

	testutil.AssertErrorResponse(t, rr, http.StatusForbidden)
}

// =============================================================================
// GET STAFF GROUPS - INVALID ID TEST
// =============================================================================

// TestVacationQuotaWriteStaysOnTheTimeTrackingTier pins the gate on
// PUT /{id}/vacation/quota (#2906): the quota belongs to the time-tracking
// tier, which also owns reading it back and the Abwesenheiten tab that shows
// it. staff:manage — the personnel-record authority — must not be able to
// write a value it can never see.
func TestVacationQuotaWriteStaysOnTheTimeTrackingTier(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)
	colleague := testpkg.CreateTestStaff(t, ctx.db, "Urlaubs", "Kontingent")
	path := fmt.Sprintf("/staff/%d/vacation/quota", colleague.ID)
	body := map[string]interface{}{"year": timezone.TodayDate().Year(), "days": 30}

	refused := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(
		t, http.MethodPut, path, body, testutil.WithJWTBearer(authToken(t, "staff:manage"))))
	assert.Equal(t, http.StatusForbidden, refused.Code,
		"staff:manage must not write a quota it cannot read: %s", refused.Body.String())

	allowed := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(
		t, http.MethodPut, path, body, testutil.WithJWTBearer(authToken(t, "time_tracking:manage"))))
	assert.Equal(t, http.StatusOK, allowed.Code, allowed.Body.String())
}
