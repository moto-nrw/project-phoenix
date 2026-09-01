package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func coverageClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return timezone.NormalizeWallClock(parsed)
}

func setupShiftCoverageRoute(t *testing.T) chi.Router {
	t.Helper()
	db, services := testutil.SetupAPITest(t)
	resource := NewResource(Dependencies{TimetableData: services.TimetableData, DB: db})
	router := chi.NewRouter()
	router.Mount("/timetable", resource.Router())
	return router
}

func createCoverageShift(t *testing.T, s *plannedConflictsSetup, staffID int64, date timezone.Date, start, end string) *scheduleModel.StaffShift {
	t.Helper()
	shift := &scheduleModel.StaffShift{
		StaffID: staffID, Date: date,
		StartTime: coverageClock(t, start), EndTime: coverageClock(t, end),
		CreatedBy: staffID,
	}
	shift.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(s.db).StaffShift.Create(s.ctx, shift))
	return shift
}

func shiftCoverageRouter(parentCtx context.Context, resource *Resource) chi.Router {
	tenantID := tenant.FromContext(parentCtx)
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			ctx := tenant.WithTenantID(testpkg.WithPackageTenantRuntime(request.Context()), tenantID)
			next.ServeHTTP(w, request.WithContext(ctx))
		})
	})
	router.Post("/shift-coverage", resource.checkShiftCoverage)
	return router
}

func postShiftCoverage(t *testing.T, router chi.Router, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/shift-coverage", bytes.NewReader(encoded))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeShiftCoverage(t *testing.T, recorder *httptest.ResponseRecorder) ShiftCoverageResponse {
	t.Helper()
	var envelope struct {
		Status string                `json:"status"`
		Data   ShiftCoverageResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope), recorder.Body.String())
	require.Equal(t, "success", envelope.Status)
	return envelope.Data
}

func TestShiftCoverage_ExactWarningAndConcreteDeviationSemantics(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	s.date = timezone.NewDate(2064, time.November, 3) // Monday
	router := shiftCoverageRouter(s.ctx, s.res)
	planned := testpkg.CreateTestStaff(t, s.db, "Absent", fmt.Sprintf("Planned-%d", time.Now().UnixNano()))
	substitute := testpkg.CreateTestStaff(t, s.db, "Active", fmt.Sprintf("Sub-%d", time.Now().UnixNano()))
	createCoverageShift(t, s, substitute.ID, s.date, "14:00", "15:00")

	instance := testpkg.CreateTestActivityInstance(t, s.db, s.date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: "Konkrete Vertretung", StartHHMM: "14:30", EndHHMM: "15:30",
	})
	testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, planned.ID, testpkg.InstanceStaffOpts{IsAbsent: true})
	testpkg.CreateTestInstanceStaff(t, s.db, instance.ID, substitute.ID, testpkg.InstanceStaffOpts{IsSubstitute: true})

	recorder := postShiftCoverage(t, router, ShiftCoverageRequest{
		Dates: []string{s.date.String()}, StartTime: "14:30", EndTime: "15:30",
		StaffIDs: []int64{planned.ID, substitute.ID}, ExcludeInstanceID: &instance.ID,
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	result := decodeShiftCoverage(t, recorder)
	require.Len(t, result.CoverageWarnings, 1, "absent planned staff is skipped; substitute is checked")
	warning := result.CoverageWarnings[0]
	assert.Equal(t, substitute.ID, warning.StaffID)
	assert.Contains(t, warning.StaffName, "Active Sub-")
	assert.Equal(t, s.date.String(), warning.Date)
	assert.Equal(t, "14:30", warning.StartTime)
	assert.Equal(t, "15:30", warning.EndTime)
	assert.Equal(t, "15:00", warning.UncoveredStartTime)
	assert.Equal(t, "15:30", warning.UncoveredEndTime)

	createCoverageShift(t, s, substitute.ID, s.date, "15:00", "15:30")
	coveredRecorder := postShiftCoverage(t, router, ShiftCoverageRequest{
		Dates: []string{s.date.String()}, StartTime: "14:30", EndTime: "15:30",
		StaffIDs: []int64{planned.ID, substitute.ID}, ExcludeInstanceID: &instance.ID,
	})
	require.Equal(t, http.StatusOK, coveredRecorder.Code, coveredRecorder.Body.String())
	assert.Empty(t, decodeShiftCoverage(t, coveredRecorder).CoverageWarnings, "adjacent shifts fully cover the active substitute")
}

func TestShiftCoverage_MultiDatePeriodAndABFiltering(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	weekA := timezone.NewDate(2065, time.November, 2) // Monday
	weekB := weekA.AddDays(7)
	router := shiftCoverageRouter(s.ctx, s.res)
	target := testpkg.CreateTestStaff(t, s.db, "Series", fmt.Sprintf("Target-%d", time.Now().UnixNano()))
	activator := testpkg.CreateTestStaff(t, s.db, "Week", fmt.Sprintf("Activator-%d", time.Now().UnixNano()))
	createCoverageShift(t, s, activator.ID, weekA, "07:00", "08:00")
	createCoverageShift(t, s, activator.ID, weekB, "07:00", "08:00")

	anchor := weekA
	period := &scheduleModel.CalendarPeriod{
		Name: fmt.Sprintf("Coverage Period %d", time.Now().UnixNano()), PeriodType: scheduleModel.PeriodTypeSchoolYear,
		StartDate: weekA, EndDate: weekB.AddDays(6), WeekCycleLength: 2, WeekCycleAnchor: &anchor, IsActive: true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(s.db).CalendarPeriod.Create(s.ctx, period))
	weekPattern := 1

	recorder := postShiftCoverage(t, router, ShiftCoverageRequest{
		Dates: []string{
			weekA.String(), weekA.AddDays(2).String(), weekA.String(),
			weekB.String(), weekB.AddDays(2).String(), weekB.AddDays(14).String(),
		},
		StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{target.ID},
		CalendarPeriodID: &period.ID, WeekPattern: &weekPattern,
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	result := decodeShiftCoverage(t, recorder)
	require.Len(t, result.CoverageWarnings, 2)
	assert.Equal(t, weekA.String(), result.CoverageWarnings[0].Date)
	assert.Equal(t, weekA.AddDays(2).String(), result.CoverageWarnings[1].Date)
}

func TestShiftCoverage_SuppressesEachUnusedWorkWeekIndependently(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	usedMonday := timezone.NewDate(2066, time.November, 1)
	unusedMonday := usedMonday.AddDays(7)
	router := shiftCoverageRouter(s.ctx, s.res)
	target := testpkg.CreateTestStaff(t, s.db, "Weekly", fmt.Sprintf("Target-%d", time.Now().UnixNano()))
	activator := testpkg.CreateTestStaff(t, s.db, "Weekly", fmt.Sprintf("Activator-%d", time.Now().UnixNano()))
	createCoverageShift(t, s, activator.ID, usedMonday, "07:00", "08:00")

	recorder := postShiftCoverage(t, router, ShiftCoverageRequest{
		Dates:     []string{usedMonday.String(), unusedMonday.String()},
		StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{target.ID},
	})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	result := decodeShiftCoverage(t, recorder)
	require.Len(t, result.CoverageWarnings, 1)
	assert.Equal(t, 1, result.CoverageWarningCount)
	assert.Equal(t, usedMonday.String(), result.CoverageWarnings[0].Date)
}

func TestShiftCoverage_ValidationAndStableErrors(t *testing.T) {
	t.Parallel()

	s := buildPlannedConflictsSetup(t)
	router := shiftCoverageRouter(s.ctx, s.res)
	validDate := timezone.NewDate(2067, time.November, 7)
	periodID := int64(77)
	weekPattern := 1
	excludeID := int64(88)
	groupID := int64(89)
	concreteDate := validDate.AddDays(1).String()
	tooManyStaff := make([]int64, 501)
	for index := range tooManyStaff {
		tooManyStaff[index] = int64(index + 1)
	}

	tests := []struct {
		name string
		body any
	}{
		{name: "dates required", body: ShiftCoverageRequest{StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{99}}},
		{name: "invalid date", body: ShiftCoverageRequest{Dates: []string{"bad"}, StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{99}}},
		{name: "end before start", body: ShiftCoverageRequest{Dates: []string{validDate.String()}, StartTime: "10:00", EndTime: "09:00", StaffIDs: []int64{99}}},
		{name: "positive staff IDs", body: ShiftCoverageRequest{Dates: []string{validDate.String()}, StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{0}}},
		{name: "staff count bound", body: ShiftCoverageRequest{Dates: []string{validDate.String()}, StartTime: "09:00", EndTime: "10:00", StaffIDs: tooManyStaff}},
		{name: "multi-date instance requires concrete date", body: ShiftCoverageRequest{Dates: []string{validDate.String(), validDate.AddDays(1).String()}, StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{99}, ExcludeInstanceID: &excludeID}},
		{name: "concrete date requires instance", body: ShiftCoverageRequest{Dates: []string{validDate.String(), concreteDate}, StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{99}, ConcreteInstanceDate: &concreteDate}},
		{name: "replan group and instance are exclusive", body: ShiftCoverageRequest{Dates: []string{validDate.String()}, StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{99}, ExcludeInstanceID: &excludeID, ReplanActivityGroupID: &groupID, CalendarPeriodID: &periodID, WeekPattern: &weekPattern}},
		{name: "period pair", body: ShiftCoverageRequest{Dates: []string{validDate.String()}, StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{99}, CalendarPeriodID: &periodID}},
		{name: "week pattern range", body: ShiftCoverageRequest{Dates: []string{validDate.String()}, StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{99}, CalendarPeriodID: &periodID, WeekPattern: func() *int { v := 3; return &v }()}},
		{name: "span bound", body: ShiftCoverageRequest{Dates: []string{validDate.String(), validDate.AddDays(367).String()}, StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{99}, CalendarPeriodID: &periodID, WeekPattern: &weekPattern}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := postShiftCoverage(t, router, test.body)
			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			assert.Contains(t, recorder.Body.String(), `"error":"invalid shift coverage request"`)
		})
	}

	t.Run("internal cause is hidden", func(t *testing.T) {
		resource := NewResource(Dependencies{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{})})
		failureRouter := shiftCoverageRouter(s.ctx, resource)
		recorder := postShiftCoverage(t, failureRouter, ShiftCoverageRequest{
			Dates: []string{validDate.String()}, StartTime: "09:00", EndTime: "10:00", StaffIDs: []int64{99},
		})
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"error":"shift coverage could not be checked"`)
		assert.NotContains(t, recorder.Body.String(), "dependencies are not wired")
	})
}

func TestShiftCoverage_RouteRequiresAllPermissionsAndLegacyConflictsStaysReadOnlyAccessible(t *testing.T) {
	t.Parallel()
	router := setupShiftCoverageRoute(t)
	claims := testutil.AdminTestClaims(999999)
	requestBody := `{"dates":["2070-11-03"],"start_time":"09:00","end_time":"10:00","staff_ids":[999999]}`

	coverageRequest := func() *http.Request {
		request := httptest.NewRequest(http.MethodPost, "/timetable/shift-coverage", strings.NewReader(requestBody))
		request.Header.Set("Content-Type", "application/json")
		return request
	}

	onlySchedulesRead := testutil.ExecuteWithAuthPermissions(t, router, coverageRequest(), claims, []string{permissions.SchedulesRead})
	assert.Equal(t, http.StatusForbidden, onlySchedulesRead.Code, onlySchedulesRead.Body.String())
	onlyShiftManage := testutil.ExecuteWithAuthPermissions(t, router, coverageRequest(), claims, []string{permissions.TimeTrackingManage})
	assert.Equal(t, http.StatusForbidden, onlyShiftManage.Code, onlyShiftManage.Body.String())
	withoutUsers := testutil.ExecuteWithAuthPermissions(t, router, coverageRequest(), claims, []string{
		permissions.SchedulesRead, permissions.TimeTrackingManage,
	})
	require.Equal(t, http.StatusForbidden, withoutUsers.Code, withoutUsers.Body.String())
	all := testutil.ExecuteWithAuthPermissions(t, router, coverageRequest(), claims, []string{
		permissions.SchedulesRead, permissions.TimeTrackingManage, permissions.UsersRead,
	})
	require.Equal(t, http.StatusOK, all.Code, all.Body.String())
	var coverageEnvelope struct {
		Data ShiftCoverageResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(all.Body.Bytes(), &coverageEnvelope))
	assert.NotNil(t, coverageEnvelope.Data.CoverageWarnings)

	legacyRequest := httptest.NewRequest(http.MethodGet,
		"/timetable/conflicts?date=2070-11-03&start_time=09:00&end_time=10:00&room_id=999999", nil)
	legacy := testutil.ExecuteWithAuthPermissions(t, router, legacyRequest, claims, []string{permissions.SchedulesRead})
	require.Equal(t, http.StatusOK, legacy.Code, legacy.Body.String())
	assert.NotContains(t, legacy.Body.String(), "coverage_warnings")
}
