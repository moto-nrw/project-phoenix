package timetracking

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func overridesPath(staffID int64) string { return fmt.Sprintf("/staff/%d/target-overrides", staffID) }

func overrideBody(start, end string, minutes int) string {
	return fmt.Sprintf(`{"start_date":%q,"end_date":%q,"daily_minutes":%d}`, start, end, minutes)
}

func decodeOverride(t *testing.T, rec *httptest.ResponseRecorder) workforce.StaffTargetOverride {
	t.Helper()
	var envelope struct {
		Data workforce.StaffTargetOverride `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), rec.Body.String())
	return envelope.Data
}

func decodeOverrides(t *testing.T, rec *httptest.ResponseRecorder) []workforce.StaffTargetOverride {
	t.Helper()
	var envelope struct {
		Data []workforce.StaffTargetOverride `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), rec.Body.String())
	return envelope.Data
}

// deleteOverride drives DELETE through the resource's router.
func (ctx *overviewAPIContext) deleteOverride(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("Authorization", "Bearer "+authToken(t, "time_tracking:manage"))
	rec := httptest.NewRecorder()
	ctx.tc.router.ServeHTTP(rec, req)
	return rec
}

func TestTargetOverridesAPI_CRUD(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	staff := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "Crud")
	path := overridesPath(staff.ID)

	rec := ctx.post(path, overrideBody("2026-10-19", "2026-10-23", 510), "time_tracking:manage")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeOverride(t, rec)
	assert.Equal(t, staff.ID, created.StaffID)
	assert.Equal(t, "2026-10-19", created.StartDate)
	assert.Equal(t, 510, created.DailyMinutes)
	require.NotNil(t, created.CreatedBy, "the editor's staff record is the author")
	assert.Equal(t, ctx.staffID, *created.CreatedBy)

	rec = ctx.get(path, "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	listed := decodeOverrides(t, rec)
	require.Len(t, listed, 1)
	assert.Equal(t, created.ID, listed[0].ID)

	rec = ctx.deleteOverride(t, fmt.Sprintf("%s/%d", path, created.ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = ctx.get(path, "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, decodeOverrides(t, rec))

	rec = ctx.deleteOverride(t, fmt.Sprintf("%s/%d", path, created.ID))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestTargetOverridesAPI_RequiresTimeTrackingManage(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	staff := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "Permission")

	rec := ctx.get(overridesPath(staff.ID), "users:read")
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.post(overridesPath(staff.ID), overrideBody("2026-10-19", "2026-10-23", 510), "time_tracking:own")
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestTargetOverridesAPI_RejectsInvalidInput(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	staff := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "Invalid")
	path := overridesPath(staff.ID)

	for name, body := range map[string]string{
		"over 12 hours":   overrideBody("2026-10-19", "2026-10-23", 721),
		"negative":        overrideBody("2026-10-19", "2026-10-23", -1),
		"inverted range":  overrideBody("2026-10-23", "2026-10-19", 60),
		"missing minutes": `{"start_date":"2026-10-19","end_date":"2026-10-23"}`,
	} {
		rec := ctx.post(path, body, "time_tracking:manage")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", name, rec.Body.String())
	}
}

func TestTargetOverridesAPI_RejectsOverlap(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	staff := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "Overlap")
	other := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "OverlapOther")
	path := overridesPath(staff.ID)

	rec := ctx.post(path, overrideBody("2026-10-19", "2026-10-23", 510), "time_tracking:manage")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	first := decodeOverride(t, rec)

	rec = ctx.post(path, overrideBody("2026-10-23", "2026-10-30", 300), "time_tracking:manage")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Sonderarbeitszeit (19.10.2026 bis 23.10.2026)")

	rec = ctx.post(path, overrideBody("2026-10-26", "2026-10-30", 300), "time_tracking:manage")
	require.Equal(t, http.StatusCreated, rec.Code, "the day after the range is free: %s", rec.Body.String())

	rec = ctx.deleteOverride(t, fmt.Sprintf("%s/%d", path, first.ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = ctx.post(path, overrideBody("2026-10-23", "2026-10-23", 300), "time_tracking:manage")
	assert.Equal(t, http.StatusCreated, rec.Code, "a deleted range frees its days: %s", rec.Body.String())

	rec = ctx.post(overridesPath(other.ID), overrideBody("2026-10-19", "2026-10-23", 495), "time_tracking:manage")
	assert.Equal(t, http.StatusCreated, rec.Code, "other staff members are independent: %s", rec.Body.String())
}

func TestTargetOverridesAPI_RejectsClosedMonth(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	staff := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "Closed")
	path := overridesPath(staff.ID)

	rec := ctx.post(path, overrideBody("2026-08-24", "2026-08-28", 300), "time_tracking:manage")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	existing := decodeOverride(t, rec)

	capability := newWorkforceCapability(t, ctx.tc.db)
	_, err := capability.RecordClosedMonth(testpkg.Ctx(t), workforce.StaffMonthBalanceSnapshot{
		StaffID: staff.ID, Year: 2026, Month: 8, ClosedAt: time.Now(), ClosedBy: ctx.staffID, Source: workforce.SnapshotSourceAdmin,
	})
	require.NoError(t, err)

	rec = ctx.post(path, overrideBody("2026-08-31", "2026-09-04", 300), "time_tracking:manage")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "August 2026 ist abgeschlossen")

	rec = ctx.deleteOverride(t, fmt.Sprintf("%s/%d", path, existing.ID))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	rec = ctx.post(path, overrideBody("2026-09-07", "2026-09-11", 300), "time_tracking:manage")
	assert.Equal(t, http.StatusCreated, rec.Code, "an open month stays writable: %s", rec.Body.String())
}

// A Sonderarbeitszeit of another school is invisible: its staff member does
// not exist for the caller, and neither does its row.
func TestTargetOverridesAPI_TenantIsolation(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	foreignStaff := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "Foreign")
	rec := ctx.post(overridesPath(foreignStaff.ID), overrideBody("2026-10-19", "2026-10-23", 510), "time_tracking:manage")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	foreign := decodeOverride(t, rec)

	t.Run("other tenant", func(t *testing.T) {
		testpkg.OwnTenant(t)
		own := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "Own")
		get := func(path string) int {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer "+authToken(t, "time_tracking:manage"))
			out := httptest.NewRecorder()
			ctx.tc.router.ServeHTTP(out, req)
			return out.Code
		}
		assert.Equal(t, http.StatusNotFound, get(overridesPath(foreignStaff.ID)))
		assert.Equal(t, http.StatusOK, get(overridesPath(own.ID)))

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%d", overridesPath(own.ID), foreign.ID), nil)
		req.Header.Set("Authorization", "Bearer "+authToken(t, "time_tracking:manage"))
		out := httptest.NewRecorder()
		ctx.tc.router.ServeHTTP(out, req)
		assert.Equal(t, http.StatusNotFound, out.Code, "a foreign row cannot be reached through an own staff member")
	})

	rec = ctx.get(overridesPath(foreignStaff.ID), "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, decodeOverrides(t, rec), 1, "the owning tenant still sees its row")
}

// Rule 15 (#2940): the list reads one staff lookup and one batch of rows,
// however many ranges exist.
func TestTargetOverridesAPI_ListQueryBudget(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	staff := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "Budget")
	path := overridesPath(staff.ID)
	create := func(start, end string) {
		rec := ctx.post(path, overrideBody(start, end, 300), "time_tracking:manage")
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	create("2026-10-05", "2026-10-09")
	create("2026-10-12", "2026-10-16")
	create("2026-10-19", "2026-10-23")

	measure := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, ctx.tc.db)
		req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(counter.Context(context.Background()))
		req.Header.Set("Authorization", "Bearer "+authToken(t, "time_tracking:manage"))
		rec := httptest.NewRecorder()
		ctx.tc.router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		queries := counter.Queries()
		counter.Stop()
		return queries
	}
	small := measure()
	create("2026-11-02", "2026-11-06")
	create("2026-11-09", "2026-11-13")
	large := measure()

	assert.Equal(t, len(small), len(large), "the list must not issue a statement per range")
	testpkg.AssertQueryBudget(t, "workforce.staff_target_overrides.list", large)
}

// The cross-staff overview runs the same Soll math over prefetched data; with
// a Sonderarbeitszeit it must still agree with the staff member's Monatskarte.
func TestTargetOverridesAPI_OverviewMatchesMonthSummary(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	staff := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "Overview")
	rec := ctx.post(overridesPath(staff.ID), overrideBody("2026-08-03", "2026-08-07", 270), "time_tracking:manage")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = ctx.get(fmt.Sprintf("/staff/%d/time-tracking/month-summary?year=2026&month=8", staff.ID), "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var detail struct {
		Data workforce.MonthSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &detail))
	assert.Equal(t, 5*270, detail.Data.TargetMinutesToDate, "a staff member without schedule owes exactly the override")

	rec = ctx.get("/staff/time-tracking/overview?year=2026&month=8", "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var overview struct {
		Data workforce.TimeTrackingOverview `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &overview))
	var row *workforce.TimeTrackingOverviewRow
	for i := range overview.Data.Rows {
		if overview.Data.Rows[i].StaffID == staff.ID {
			row = &overview.Data.Rows[i]
		}
	}
	require.NotNil(t, row, "staff member must be listed")
	assert.Equal(t, detail.Data.TargetMinutesToDate, row.SollMinutes)
	assert.Equal(t, detail.Data.ClosingBalanceMinutes, row.BalanceMinutes)

	rec = ctx.get(fmt.Sprintf("/staff/%d/time-tracking/schedule-targets?from=2026-08-07&to=2026-08-10", staff.ID), "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var days struct {
		Data []workforce.DailyProjection `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &days))
	require.Len(t, days.Data, 4)
	assert.Equal(t, workforce.TargetSourceOverride, days.Data[0].TargetSource)
	assert.Equal(t, 270, days.Data[0].TargetMinutes)
	assert.Empty(t, days.Data[3].TargetSource, "the Monday after the range is a regular day")
}

// The weekly history summaries re-price a Sonderarbeitszeit week with the
// Monatskarte's daily Soll: a staff member without a schedule owes exactly the
// override in that week, and the week after has no target again.
func TestTargetOverridesAPI_HistoryWeekUsesOverrideSoll(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	staff := testpkg.CreateTestStaff(t, ctx.tc.db, "Sonder", "History")
	rec := ctx.post(overridesPath(staff.ID), overrideBody("2026-08-03", "2026-08-07", 270), "time_tracking:manage")
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	insertWorkSession(t, ctx.tc, staff.ID, "2026-08-04",
		time.Date(2026, time.August, 4, 8, 0, 0, 0, berlin), time.Date(2026, time.August, 4, 12, 30, 0, 0, berlin))
	insertWorkSession(t, ctx.tc, staff.ID, "2026-08-11",
		time.Date(2026, time.August, 11, 8, 0, 0, 0, berlin), time.Date(2026, time.August, 11, 12, 30, 0, 0, berlin))

	rec = ctx.get(fmt.Sprintf("/staff/%d/time-tracking/history?from=2026-08-03&to=2026-08-16", staff.ID), "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var history struct {
		Data struct {
			WeeklySummaries []workforce.WorkWeekSummary `json:"weekly_summaries"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &history), rec.Body.String())
	targets := map[int]*int{}
	for _, week := range history.Data.WeeklySummaries {
		targets[week.WeekNumber] = week.TargetMinutes
	}
	require.Contains(t, targets, 32)
	require.NotNil(t, targets[32], "the override week has a Soll")
	assert.Equal(t, 5*270, *targets[32])
	assert.Nil(t, targets[33], "without schedule or override there is no weekly Soll")
}
