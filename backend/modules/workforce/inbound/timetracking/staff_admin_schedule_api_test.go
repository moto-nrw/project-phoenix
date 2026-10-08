package timetracking

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Weekday indices of the schedule contract (Monday = 0), mirroring the
// capability's DayOfWeek.
const (
	dayMonday  = 0
	dayTuesday = 1
)

func TestUpdateSchedule_SaveAsTemplateMaterializesAssignedSnapshot(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff := testpkg.CreateTestStaff(t, ctx.db, "ScheduleTemplate", "Clean")
	capability := newWorkforceCapability(t, ctx.db)

	require.NoError(t, capability.ReplaceStaffSchedule(testpkg.Ctx(t), workforce.ReplaceStaffSchedule{
		StaffID: staff.ID,
		Entries: []workforce.StaffWorkScheduleEntry{
			{
				WeekIndex:      0,
				RotationLength: 1,
				DayOfWeek:      dayMonday,
				TargetMinutes:  300,
			},
		},
	}))

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
				"day_of_week":    dayTuesday,
				"target_minutes": 360,
			},
		},
	}

	req := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/staff/%d/schedule", staff.ID), body, testutil.WithJWTBearer(token))
	rr := testutil.ExecuteRequest(ctx.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	activeRows, err := capability.CurrentStaffSchedule(testpkg.Ctx(t), staff.ID)
	require.NoError(t, err)
	require.Len(t, activeRows, 1)
	assert.Equal(t, dayTuesday, activeRows[0].DayOfWeek)
	assert.Equal(t, 360, activeRows[0].TargetMinutes)

	reloadedStaff, err := newWorkforceTestRepositories(t, ctx.db).Staff.FindByID(testpkg.Ctx(t), staff.ID)
	require.NoError(t, err)
	require.NotNil(t, reloadedStaff.WorkTimeModelID)
	// config.work_time_model_entries has no tenant of its own; it only goes
	// away with its model, so the delete has to actually happen (#2419).
	t.Cleanup(func() {
		_, err := ctx.db.NewUpdate().TableExpr("users.staff_employment_profiles").
			Set("work_time_model_id = NULL").
			Where("work_time_model_id = ?", *reloadedStaff.WorkTimeModelID).
			Exec(context.Background())
		require.NoError(t, err)
		_, err = ctx.db.NewDelete().TableExpr("config.work_time_models").
			Where("id = ?", *reloadedStaff.WorkTimeModelID).
			Exec(context.Background())
		require.NoError(t, err)
	})

	model, err := capability.FindWorkTimeModel(testpkg.Ctx(t), *reloadedStaff.WorkTimeModelID)
	require.NoError(t, err)
	require.Len(t, model.Entries, 1)
	assert.Equal(t, dayTuesday, model.Entries[0].DayOfWeek)
	assert.Equal(t, 360, model.Entries[0].TargetMinutes)
}

func TestGetSchedule_AllowsOwnStaffWithTimeTrackingOwn(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "ScheduleOwn", "Read")
	capability := newWorkforceCapability(t, ctx.db)

	require.NoError(t, capability.ReplaceStaffSchedule(testpkg.Ctx(t), workforce.ReplaceStaffSchedule{
		StaffID: staff.ID,
		Entries: []workforce.StaffWorkScheduleEntry{
			{
				WeekIndex:      0,
				RotationLength: 1,
				DayOfWeek:      dayMonday,
				TargetMinutes:  300,
			},
		},
	}))

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
	body := map[string]interface{}{"year": testpkg.TodayDate().Year(), "entitled_days": 30, "reason": "Tarif"}

	refused := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(
		t, http.MethodPut, path, body, testutil.WithJWTBearer(authToken(t, "staff:manage"))))
	assert.Equal(t, http.StatusForbidden, refused.Code,
		"staff:manage must not write a quota it cannot read: %s", refused.Body.String())

	// The change is recorded against the editing staff member (#3256), so
	// the allowed caller is a real Leitung, not a bare token.
	leitungPerson, leitungAccount := testpkg.CreateTestPersonWithAccount(t, ctx.db, "Urlaubs", "Leitung")
	testpkg.CreateTestStaffForPerson(t, ctx.db, leitungPerson.ID)
	claims := testutil.DefaultTestClaims()
	claims.ID = int(leitungAccount.ID)
	claims.Permissions = []string{"time_tracking:manage"}
	leitung := testutil.MintTestJWT(t, claims)

	allowed := testutil.ExecuteRequest(ctx.router, testutil.NewAuthenticatedRequest(
		t, http.MethodPut, path, body, testutil.WithJWTBearer(leitung)))
	assert.Equal(t, http.StatusOK, allowed.Code, allowed.Body.String())
}

// A school that starts with moto mid-year enters the past as Nachträge; the
// first schedule has to reach back to them, or every past day is priced at a
// Soll of 0 and the Saldo shows the whole Ist (#3892).
func TestUpdateSchedule_FirstScheduleStartsOnValidFrom(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)
	staff := testpkg.CreateTestStaff(t, ctx.db, "ScheduleBackdated", "First")
	capability := newWorkforceCapability(t, ctx.db)

	claims := testutil.DefaultTestClaims()
	claims.Permissions = []string{"time_tracking:manage"}
	token := testutil.MintTestJWT(t, claims)
	put := func(validFrom string) int {
		body := map[string]any{
			"mode":            "custom",
			"rotation_length": 1,
			"valid_from":      validFrom,
			"entries": []map[string]any{
				{"week_index": 0, "day_of_week": dayMonday, "target_minutes": 480},
			},
		}
		req := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/staff/%d/schedule", staff.ID), body, testutil.WithJWTBearer(token))
		return testutil.ExecuteRequest(ctx.router, req).Code
	}

	assert.Equal(t, http.StatusBadRequest, put("2999-01-04"), "a future start is not supported")
	require.Equal(t, http.StatusOK, put("2025-09-01"))

	rows, err := capability.CurrentStaffSchedule(testpkg.Ctx(t), staff.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2025-09-01", rows[0].ValidFrom)

	// The first version now prices those days; moving its start again would
	// change a Soll that was already shown.
	assert.Equal(t, http.StatusBadRequest, put("2025-08-01"))
	rows, err = capability.CurrentStaffSchedule(testpkg.Ctx(t), staff.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2025-09-01", rows[0].ValidFrom)
}

func TestUpdateSchedule_RejectsFutureValidFromForEmptyCustomSchedule(t *testing.T) {
	t.Parallel()

	ctx := setupStaffRoute(t)
	capability := newWorkforceCapability(t, ctx.db)
	claims := testutil.DefaultTestClaims()
	claims.Permissions = []string{"time_tracking:manage"}
	token := testutil.MintTestJWT(t, claims)

	for _, test := range []struct {
		name    string
		entries []map[string]any
	}{
		{name: "no entries", entries: []map[string]any{}},
		{name: "only zero targets", entries: []map[string]any{{"week_index": 0, "day_of_week": dayMonday, "target_minutes": 0}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			staff := testpkg.CreateTestStaff(t, ctx.db, "ScheduleFuture", test.name)
			require.NoError(t, capability.ReplaceStaffSchedule(testpkg.Ctx(t), workforce.ReplaceStaffSchedule{
				StaffID: staff.ID,
				Entries: []workforce.StaffWorkScheduleEntry{{
					WeekIndex: 0, RotationLength: 1, DayOfWeek: dayMonday, TargetMinutes: 480,
				}},
			}))

			body := map[string]any{
				"mode": "custom", "rotation_length": 1, "valid_from": "2999-01-04", "entries": test.entries,
			}
			req := testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/staff/%d/schedule", staff.ID), body, testutil.WithJWTBearer(token))
			rr := testutil.ExecuteRequest(ctx.router, req)
			assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

			rows, err := capability.CurrentStaffSchedule(testpkg.Ctx(t), staff.ID)
			require.NoError(t, err)
			require.Len(t, rows, 1, "the rejected write must not close the current schedule")
			assert.Equal(t, 480, rows[0].TargetMinutes)
		})
	}
}
