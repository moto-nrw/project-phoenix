package staffshifts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() { testutil.SeedTestJWTConfig() }

type fakeOverviewService struct {
	result *scheduleSvc.StaffScheduleOverview
	err    error
	from   timezone.Date
	to     timezone.Date
}

func (f *fakeOverviewService) GetOverview(_ context.Context, from, to timezone.Date) (*scheduleSvc.StaffScheduleOverview, error) {
	f.from, f.to = from, to
	return f.result, f.err
}

func overviewClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return timezone.NormalizeWallClock(parsed)
}

func TestOverviewHandler_StableWireContract(t *testing.T) {
	t.Parallel()

	from := timezone.NewDate(2026, time.July, 6)
	to := from.AddDays(4)
	member := &users.Staff{Person: &users.Person{FirstName: "Ada", LastName: "Lovelace"}}
	member.ID = 7
	shift := &scheduleModel.StaffShift{
		StaffID: 7, Date: scheduleModel.Date(from), StartTime: overviewClock(t, "08:00"), EndTime: overviewClock(t, "12:00"), BreakMinutes: 30,
	}
	shift.ID = 8
	service := &fakeOverviewService{result: &scheduleSvc.StaffScheduleOverview{
		From: from, To: to, DienstplanInUse: true,
		Staff:  []*users.Staff{member},
		Shifts: []*scheduleModel.StaffShift{shift},
		Assignments: []scheduleSvc.StaffScheduleAssignment{{
			InstanceID: 9, StaffID: 7, Date: from,
			StartTime: overviewClock(t, "09:00"), EndTime: overviewClock(t, "11:00"),
			ActivityTitle: "Lernzeit", RoomID: 10, RoomName: "Blau", Status: scheduleModel.InstanceStatusPlanned,
			CoverageStatus: scheduleSvc.CoverageStatusUncovered,
			UncoveredIntervals: []scheduleSvc.ShiftCoverageInterval{{
				StartTime: overviewClock(t, "10:30"), EndTime: overviewClock(t, "11:00"),
			}},
		}},
		WeeklySummaries: []scheduleSvc.StaffWeeklySummary{
			{StaffID: 7, WeekStart: from, PlannedMinutes: 210, TargetMinutes: testpkg.IntPtr(1215), DeltaMinutes: testpkg.IntPtr(-1005)},
			{StaffID: 11, WeekStart: from, PlannedMinutes: 180},
		},
	}}
	resource := &Resource{Overview: service}
	request := httptest.NewRequest(http.MethodGet, "/overview?from=2026-07-06&to=2026-07-10", nil)
	recorder := httptest.NewRecorder()

	resource.overview(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, from, service.from)
	assert.Equal(t, to, service.to)

	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	assert.Equal(t, "2026-07-06", envelope.Data["from"])
	assert.Equal(t, "2026-07-10", envelope.Data["to"])
	assert.Equal(t, true, envelope.Data["dienstplan_in_use"])
	require.IsType(t, []any{}, envelope.Data["staff"])
	require.IsType(t, []any{}, envelope.Data["shifts"])
	require.IsType(t, []any{}, envelope.Data["assignments"])
	assignments := envelope.Data["assignments"].([]any)
	require.Len(t, assignments, 1)
	assignment := assignments[0].(map[string]any)
	assert.Contains(t, assignment, "absence_reason")
	assert.Nil(t, assignment["absence_reason"])
	assert.Contains(t, assignment, "coverage_reason")
	assert.Nil(t, assignment["coverage_reason"])
	assert.Equal(t, "uncovered", assignment["coverage_status"])
	intervals := assignment["uncovered_intervals"].([]any)
	require.Len(t, intervals, 1)
	assert.Equal(t, "10:30", intervals[0].(map[string]any)["start_time"])
	assert.Equal(t, "11:00", intervals[0].(map[string]any)["end_time"])

	require.IsType(t, []any{}, envelope.Data["weekly_summaries"])
	weeklySummaries := envelope.Data["weekly_summaries"].([]any)
	require.Len(t, weeklySummaries, 2)
	withTarget := weeklySummaries[0].(map[string]any)
	assert.Equal(t, float64(7), withTarget["staff_id"])
	assert.Equal(t, "2026-07-06", withTarget["week_start"])
	assert.Equal(t, float64(210), withTarget["planned_minutes"])
	assert.Equal(t, float64(1215), withTarget["target_minutes"])
	assert.Equal(t, float64(-1005), withTarget["delta_minutes"])
	withoutTarget := weeklySummaries[1].(map[string]any)
	assert.Contains(t, withoutTarget, "target_minutes")
	assert.Nil(t, withoutTarget["target_minutes"])
	assert.Contains(t, withoutTarget, "delta_minutes")
	assert.Nil(t, withoutTarget["delta_minutes"])
}

func TestOverviewHandler_EmptyArraysStayNonNull(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.July, 6)
	resource := &Resource{Overview: &fakeOverviewService{result: &scheduleSvc.StaffScheduleOverview{From: date, To: date}}}
	recorder := httptest.NewRecorder()
	resource.overview(recorder, httptest.NewRequest(http.MethodGet, "/overview?from=2026-07-06&to=2026-07-06", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var envelope struct {
		Data OverviewResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	assert.NotNil(t, envelope.Data.Staff)
	assert.NotNil(t, envelope.Data.Shifts)
	assert.NotNil(t, envelope.Data.Assignments)
	assert.NotNil(t, envelope.Data.WeeklySummaries)
}

func TestOverviewHandler_RejectsBadRangeAndPropagatesServiceFailure(t *testing.T) {
	t.Parallel()

	t.Run("bad date", func(t *testing.T) {
		resource := &Resource{Overview: &fakeOverviewService{}}
		recorder := httptest.NewRecorder()
		resource.overview(recorder, httptest.NewRequest(http.MethodGet, "/overview?from=bad&to=2026-07-10", nil))
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "inverted range", err: fmt.Errorf("%w: end before start", scheduleSvc.ErrShiftInvalid)},
		{name: "oversized range", err: scheduleSvc.ErrShiftRangeTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			resource := &Resource{Overview: &fakeOverviewService{err: test.err}}
			recorder := httptest.NewRecorder()
			resource.overview(recorder, httptest.NewRequest(http.MethodGet, "/overview?from=2026-07-06&to=2026-07-10", nil))
			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		})
	}

	t.Run("read failure", func(t *testing.T) {
		const rawCause = "sentinel raw SQL cause: relation schedule.staff_shifts does not exist"
		resource := &Resource{Overview: &fakeOverviewService{err: errors.New(rawCause)}}
		recorder := httptest.NewRecorder()
		resource.overview(recorder, httptest.NewRequest(http.MethodGet, "/overview?from=2026-07-06&to=2026-07-10", nil))
		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"error":"staff schedule overview could not be loaded"`)
		assert.NotContains(t, recorder.Body.String(), rawCause)
	})
}

func TestOverviewRoute_RequiresScheduleShiftAndUserPermissions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	from := timezone.NewDate(2070, time.November, 3)
	to := from.AddDays(4)
	resource := &Resource{
		Overview: &fakeOverviewService{result: &scheduleSvc.StaffScheduleOverview{From: from, To: to}},
		db:       db,
	}
	router := chi.NewRouter()
	router.Mount("/api/staff-shifts", resource.Router())
	claims := testutil.AdminTestClaims(999999)
	request := func() *http.Request {
		return httptest.NewRequest(http.MethodGet,
			"/api/staff-shifts/overview?from=2070-11-03&to=2070-11-07", nil)
	}

	shiftOnly := testutil.ExecuteWithAuthPermissions(t, router, request(), claims, []string{permissions.TimeTrackingManage})
	assert.Equal(t, http.StatusForbidden, shiftOnly.Code, shiftOnly.Body.String())
	scheduleOnly := testutil.ExecuteWithAuthPermissions(t, router, request(), claims, []string{permissions.SchedulesRead})
	assert.Equal(t, http.StatusForbidden, scheduleOnly.Code, scheduleOnly.Body.String())
	withoutUsers := testutil.ExecuteWithAuthPermissions(t, router, request(), claims, []string{
		permissions.TimeTrackingManage,
		permissions.SchedulesRead,
	})
	assert.Equal(t, http.StatusForbidden, withoutUsers.Code, withoutUsers.Body.String())
	all := testutil.ExecuteWithAuthPermissions(t, router, request(), claims, []string{
		permissions.TimeTrackingManage,
		permissions.SchedulesRead,
		permissions.UsersRead,
	})
	assert.Equal(t, http.StatusOK, all.Code, all.Body.String())
}
