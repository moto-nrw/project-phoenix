package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// Soll und Ist je Kind (#4774) through the production router: the care plan
// supplies the Soll, active.attendance the Ist.

type careTimeDayPayload struct {
	Date             string  `json:"date"`
	Kind             string  `json:"kind"`
	Overrun          bool    `json:"overrun"`
	PlannedStart     *string `json:"planned_start"`
	PlannedEnd       *string `json:"planned_end"`
	ActualStart      *string `json:"actual_start"`
	ActualEnd        *string `json:"actual_end"`
	DeviationMinutes *int    `json:"deviation_minutes"`
	EarlyMinutes     int     `json:"early_minutes"`
}

type careTimeSummaryPayload struct {
	OverrunCount int `json:"overrun_count"`
	UnbookedDays int `json:"unbooked_days"`
	ExtraMinutes int `json:"extra_minutes"`
}

type studentCareTimesPayload struct {
	Data struct {
		StudentID        string                 `json:"student_id"`
		ToleranceMinutes int                    `json:"tolerance_minutes"`
		Summary          careTimeSummaryPayload `json:"summary"`
		Days             []careTimeDayPayload   `json:"days"`
	} `json:"data"`
}

type careTimeOverrunsPayload struct {
	Data struct {
		Students []struct {
			StudentID string                 `json:"student_id"`
			LastName  string                 `json:"last_name"`
			Summary   careTimeSummaryPayload `json:"summary"`
		} `json:"students"`
	} `json:"data"`
}

func insertAttendanceSpan(t *testing.T, db *bun.DB, tenantID, studentID, deviceID int64, date calendar.Date, from, to time.Duration) {
	t.Helper()
	checkIn := date.BerlinMidnight().Add(from)
	checkOut := date.BerlinMidnight().Add(to)
	testpkg.CreateTestAttendanceForTenant(t, db, tenantID, studentID, 0, deviceID, date, checkIn, &checkOut)
}

// careTimeChild is booked on Mondays 08:00 to 14:00 and attended a booked
// Monday as planned, an unbooked Tuesday and a later Monday an hour early.
func careTimeChild(t *testing.T, tc *statisticsHTTPTestContext, staffID int64) int64 {
	t.Helper()
	tenantID := testpkg.Tenant(t)
	child := testpkg.CreateTestStudent(t, tc.db, "Sina", "Soll", "3b")
	testpkg.CreateTestArrivalSchedule(t, tc.db, child.ID, 1, staffID, "08:00")
	testpkg.CreateTestPickupSchedule(t, tc.db, child.ID, 1, staffID, "14:00")
	device := testpkg.CreateTestDevice(t, tc.db, "care-time-device")
	insertAttendanceSpan(t, tc.db, tenantID, child.ID, device.ID, calendar.NewDate(2026, 6, 8), 8*time.Hour, 14*time.Hour)
	insertAttendanceSpan(t, tc.db, tenantID, child.ID, device.ID, calendar.NewDate(2026, 6, 9), 8*time.Hour, 14*time.Hour)
	insertAttendanceSpan(t, tc.db, tenantID, child.ID, device.ID, calendar.NewDate(2026, 6, 15), 7*time.Hour, 14*time.Hour)
	return child.ID
}

const careTimeWindow = "from=2026-06-08&to=2026-06-16"

func TestStudentCareTimes_ComparesTheCarePlanWithTheAttendance(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, testpkg.Tenant(t), "Soll", "Leser")
	childID := careTimeChild(t, tc, staff.ID)

	req := httptest.NewRequest(http.MethodGet, "/students/"+strconv.FormatInt(childID, 10)+"/care-times?"+careTimeWindow, nil)
	rec := authExec(t, tc, req, claimsFor(t, account.ID), []string{permissions.UsersRead})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload studentCareTimesPayload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	data := payload.Data
	assert.Equal(t, 15, data.ToleranceMinutes)
	require.Len(t, data.Days, 3, "days neither booked nor attended are not listed")

	assert.Equal(t, "2026-06-08", data.Days[0].Date)
	assert.Equal(t, "within_plan", data.Days[0].Kind)
	require.NotNil(t, data.Days[0].PlannedStart)
	assert.Equal(t, "08:00", *data.Days[0].PlannedStart)
	assert.Equal(t, "14:00", *data.Days[0].PlannedEnd)
	assert.Equal(t, "08:00", *data.Days[0].ActualStart, "the check-in shows as Berlin wall clock")
	assert.Equal(t, 0, *data.Days[0].DeviationMinutes)

	assert.Equal(t, "unbooked", data.Days[1].Kind)
	assert.True(t, data.Days[1].Overrun)
	assert.Nil(t, data.Days[1].PlannedStart)

	assert.Equal(t, "2026-06-15", data.Days[2].Date)
	assert.Equal(t, "overrun", data.Days[2].Kind)
	assert.Equal(t, 60, data.Days[2].EarlyMinutes)

	assert.Equal(t, careTimeSummaryPayload{OverrunCount: 2, UnbookedDays: 1, ExtraMinutes: 60 + 360}, data.Summary)
}

func TestCareTimeOverruns_ListsAndExportsChildrenWithOverruns(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, testpkg.Tenant(t), "Soll", "Leitung")
	childID := careTimeChild(t, tc, staff.ID)
	claims := claimsFor(t, account.ID)

	req := httptest.NewRequest(http.MethodGet, "/care-time-overruns?"+careTimeWindow, nil)
	rec := authExec(t, tc, req, claims, reportPermissions)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload careTimeOverrunsPayload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Students, 1)
	assert.Equal(t, strconv.FormatInt(childID, 10), payload.Data.Students[0].StudentID)
	assert.Equal(t, 2, payload.Data.Students[0].Summary.OverrunCount)

	for _, format := range []string{"pdf", "xlsx"} {
		req := httptest.NewRequest(http.MethodGet, "/care-time-overruns/export?"+careTimeWindow+"&format="+format, nil)
		rec := authExec(t, tc, req, claims, reportPermissions)
		require.Equal(t, http.StatusOK, rec.Code, format)
		assert.Contains(t, rec.Header().Get("Content-Disposition"), "soll-und-ist-2026-06-08-2026-06-16."+format)
		assert.NotEmpty(t, rec.Body.Bytes())
	}
	req = httptest.NewRequest(http.MethodGet, "/care-time-overruns/export?"+careTimeWindow+"&format=docx", nil)
	assert.Equal(t, http.StatusBadRequest, authExec(t, tc, req, claims, reportPermissions).Code)
}

// TestCareTimesQueryBudgets keeps the three comparison reads flat: the child
// view must not add a query for each listed day, and the list and export must
// not add one for each child.
func TestCareTimesQueryBudgets(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, testpkg.Tenant(t), "Budget", "Leitung")
	claims := claimsFor(t, account.ID)
	childID := careTimeChild(t, tc, staff.ID)

	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	type measurement struct {
		statements []string
		body       []byte
	}
	run := func(path string) measurement {
		counter.Reset()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := authExec(t, tc, req.WithContext(counter.Context(req.Context())), claims, reportPermissions)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return measurement{statements: counter.Queries(), body: rec.Body.Bytes()}
	}

	budgetWindow := "from=2026-06-08&to=2026-06-30"
	studentPath := "/students/" + strconv.FormatInt(childID, 10) + "/care-times?" + budgetWindow
	studentOne := run(studentPath)
	overrunsOne := run("/care-time-overruns?" + budgetWindow)
	exportOne := run("/care-time-overruns/export?" + budgetWindow + "&format=xlsx")
	var studentPayload studentCareTimesPayload
	require.NoError(t, json.Unmarshal(studentOne.body, &studentPayload))
	require.Len(t, studentPayload.Data.Days, 5)
	var overrunPayload careTimeOverrunsPayload
	require.NoError(t, json.Unmarshal(overrunsOne.body, &overrunPayload))
	require.Len(t, overrunPayload.Data.Students, 1)
	require.NotEmpty(t, exportOne.body)
	// The first list and export record their access. Measure the steady-state
	// read separately so that audit insertion does not mask a batch regression.
	overrunsBaseline := run("/care-time-overruns?" + budgetWindow)
	exportBaseline := run("/care-time-overruns/export?" + budgetWindow + "&format=xlsx")

	device := testpkg.CreateTestDevice(t, tc.db, "care-time-budget-device")
	for _, date := range []calendar.Date{calendar.NewDate(2026, 6, 22), calendar.NewDate(2026, 6, 23), calendar.NewDate(2026, 6, 29)} {
		insertAttendanceSpan(t, tc.db, testpkg.Tenant(t), childID, device.ID, date, 8*time.Hour, 14*time.Hour)
	}

	for i := 0; i < 3; i++ {
		careTimeChild(t, tc, staff.ID)
	}
	studentMany := run(studentPath)
	overrunsMany := run("/care-time-overruns?" + budgetWindow)
	exportMany := run("/care-time-overruns/export?" + budgetWindow + "&format=xlsx")
	require.NoError(t, json.Unmarshal(studentMany.body, &studentPayload))
	require.Len(t, studentPayload.Data.Days, 6)
	require.NoError(t, json.Unmarshal(overrunsMany.body, &overrunPayload))
	require.Len(t, overrunPayload.Data.Students, 4)
	require.NotEmpty(t, exportMany.body)

	assert.Len(t, studentMany.statements, len(studentOne.statements), "the child view must batch its day inputs")
	assert.Len(t, overrunsMany.statements, len(overrunsBaseline.statements), "the overrun list must batch its children")
	assert.Len(t, exportMany.statements, len(exportBaseline.statements), "the export must reuse the batched overrun list")
	querybudgets.AssertQueryBudget(t, "api.statistics.student_care_times", studentMany.statements)
	querybudgets.AssertQueryBudget(t, "api.statistics.care_time_overruns", overrunsMany.statements)
	querybudgets.AssertQueryBudget(t, "api.statistics.care_time_overruns_export", exportMany.statements)
}

func TestCareTimes_RejectMissingRightsWithARegisteredCode(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	staff, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, testpkg.Tenant(t), "Ohne", "Recht")
	childID := careTimeChild(t, tc, staff.ID)
	claims := claimsFor(t, account.ID)

	for path, perms := range map[string][]string{
		"/students/" + strconv.FormatInt(childID, 10) + "/care-times?" + careTimeWindow: {permissions.ConfigRead},
		"/care-time-overruns?" + careTimeWindow:                                         {permissions.UsersRead},
		"/care-time-overruns/export?" + careTimeWindow:                                  {permissions.UsersRead},
	} {
		rec := authExec(t, tc, httptest.NewRequest(http.MethodGet, path, nil), claims, perms)
		assert.Equal(t, http.StatusForbidden, rec.Code, path)
		assert.Contains(t, rec.Body.String(), `"code":"general.permission"`, path)
	}
}

func TestStudentCareTimes_HidesAChildOfAnotherSchool(t *testing.T) {
	t.Parallel()
	tc := setupStatisticsRoute(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, testpkg.Tenant(t), "Fremde", "Schule")
	otherTenant, _ := testpkg.CreateTestTenant(t, tc.db)
	foreign := testpkg.CreateTestStudentForTenant(t, tc.db, otherTenant, "Fremdes", "Kind", "1a")

	req := httptest.NewRequest(http.MethodGet, "/students/"+strconv.FormatInt(foreign.ID, 10)+"/care-times?"+careTimeWindow, nil)
	rec := authExec(t, tc, req, claimsFor(t, account.ID), []string{permissions.UsersRead})
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}
