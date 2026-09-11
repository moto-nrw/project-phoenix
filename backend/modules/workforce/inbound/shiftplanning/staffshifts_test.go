package shiftplanning

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The staff-shift route is composed here, so the permission gates, the actor
// resolution, the payload mapping and the failure envelope are asserted
// against the real runtime over a planning contract fake.

// fakePlanning records the capability calls the handlers make.
type fakePlanning struct {
	createFn   func(context.Context, workforce.CreateStaffShift) (workforce.PlannedShift, error)
	moveFn     func(context.Context, workforce.MoveStaffShift) (workforce.PlannedShift, error)
	cancelFn   func(context.Context, workforce.CancelStaffShift) (workforce.StaffShiftCancellation, error)
	seriesFn   func(context.Context, workforce.CreateStaffShiftSeries) (workforce.StaffShiftSeriesResult, error)
	splitFn    func(context.Context, workforce.SplitStaffShiftSeries) (workforce.StaffShiftSeriesResult, error)
	getFn      func(context.Context, int64) (workforce.StaffShiftSeries, error)
	endFn      func(context.Context, int64, string) (workforce.StaffShiftSeriesResult, error)
	overviewFn func(context.Context, string, string) (workforce.StaffScheduleOverview, error)
	exportFn   func(context.Context, workforce.PlanExportRequest) (workforce.PlanExportFile, error)
}

func (f *fakePlanning) ListShifts(context.Context, workforce.ShiftRange) ([]workforce.PlannedShift, error) {
	return []workforce.PlannedShift{}, nil
}

func (f *fakePlanning) CreateShift(ctx context.Context, input workforce.CreateStaffShift) (workforce.PlannedShift, error) {
	if f.createFn != nil {
		return f.createFn(ctx, input)
	}
	return workforce.PlannedShift{}, nil
}

func (f *fakePlanning) UpdateShift(context.Context, workforce.UpdateStaffShift) (workforce.PlannedShift, error) {
	return workforce.PlannedShift{}, nil
}

func (f *fakePlanning) MoveShift(ctx context.Context, input workforce.MoveStaffShift) (workforce.PlannedShift, error) {
	if f.moveFn != nil {
		return f.moveFn(ctx, input)
	}
	return workforce.PlannedShift{}, nil
}

func (f *fakePlanning) ApplyCancellation(ctx context.Context, input workforce.CancelStaffShift) (workforce.StaffShiftCancellation, error) {
	if f.cancelFn != nil {
		return f.cancelFn(ctx, input)
	}
	return workforce.StaffShiftCancellation{}, nil
}

func (f *fakePlanning) DeleteShift(context.Context, int64) error { return nil }

func (f *fakePlanning) CreateSeries(ctx context.Context, input workforce.CreateStaffShiftSeries) (workforce.StaffShiftSeriesResult, error) {
	if f.seriesFn != nil {
		return f.seriesFn(ctx, input)
	}
	return workforce.StaffShiftSeriesResult{}, errors.New("seriesFn not set")
}

func (f *fakePlanning) SplitSeries(ctx context.Context, input workforce.SplitStaffShiftSeries) (workforce.StaffShiftSeriesResult, error) {
	if f.splitFn != nil {
		return f.splitFn(ctx, input)
	}
	return workforce.StaffShiftSeriesResult{}, errors.New("splitFn not set")
}

func (f *fakePlanning) GetSeries(ctx context.Context, id int64) (workforce.StaffShiftSeries, error) {
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return workforce.StaffShiftSeries{}, errors.New("getFn not set")
}

func (f *fakePlanning) EndSeries(ctx context.Context, id int64, from string) (workforce.StaffShiftSeriesResult, error) {
	if f.endFn != nil {
		return f.endFn(ctx, id, from)
	}
	return workforce.StaffShiftSeriesResult{}, errors.New("endFn not set")
}

func (f *fakePlanning) Overview(ctx context.Context, from, to string) (workforce.StaffScheduleOverview, error) {
	if f.overviewFn != nil {
		return f.overviewFn(ctx, from, to)
	}
	return workforce.StaffScheduleOverview{From: from, To: to}, nil
}

func (f *fakePlanning) ExportPlan(ctx context.Context, request workforce.PlanExportRequest) (workforce.PlanExportFile, error) {
	if f.exportFn != nil {
		return f.exportFn(ctx, request)
	}
	return workforce.PlanExportFile{}, errors.New("exportFn not set")
}

const testActorStaffID = int64(42)

type staffShiftsRoute struct {
	router chi.Router
	token  string
	claims func(...string) string
}

// setupStaffShiftsRoute composes the route over the fake with an actor that
// resolves to staff 42 and account 7, the way the JWT claims normally drive.
func setupStaffShiftsRoute(t *testing.T, planning workforce.StaffShiftPlanning) *staffShiftsRoute {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	resource := NewStaffShiftsResource(StaffShiftsDependencies{
		Planning:       planning,
		DB:             db,
		ResolveStaffID: func(context.Context) (int64, error) { return testActorStaffID, nil },
		ActorAccountID: func(context.Context) *int64 { var account int64 = 7; return &account },
	})
	mint := func(granted ...string) string {
		claims := testutil.DefaultTestClaims()
		claims.TenantID = testpkg.Tenant(t)
		claims.Permissions = granted
		claims.IsAdmin = false
		return testutil.MintTestJWT(t, claims)
	}
	return &staffShiftsRoute{router: resource.Router(), token: mint(permissions.TimeTrackingManage), claims: mint}
}

func (s *staffShiftsRoute) do(t *testing.T, method, path, body string) *http.Response {
	t.Helper()
	var payload any
	if body != "" {
		payload = json.RawMessage(body)
	}
	request := testutil.NewAuthenticatedRequest(t, method, path, payload, testutil.WithJWTBearer(s.token))
	return testutil.ExecuteRequest(s.router, request).Result()
}

func responseBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer func() { _ = response.Body.Close() }()
	var builder strings.Builder
	buffer := make([]byte, 4096)
	for {
		n, err := response.Body.Read(buffer)
		builder.Write(buffer[:n])
		if err != nil {
			break
		}
	}
	return builder.String()
}

func TestStaffShiftsRouteCreateKeepsLargeOriginIDs(t *testing.T) {
	t.Parallel()
	var got workforce.CreateStaffShift
	fake := &fakePlanning{createFn: func(_ context.Context, input workforce.CreateStaffShift) (workforce.PlannedShift, error) {
		got = input
		return workforce.PlannedShift{StaffShift: workforce.StaffShift{ID: 9223372036854775806, StaffID: input.StaffID, Date: input.Date, StartTime: input.StartTime, EndTime: input.EndTime}}, nil
	}}
	route := setupStaffShiftsRoute(t, fake)

	response := route.do(t, http.MethodPost, "/", `{"staff_id":7,"date":"2026-07-07","start_time":"09:00","end_time":"15:00","break_minutes":20,"shift_type_id":null,"origin_shift_id":"9223372036854775807"}`)
	body := responseBody(t, response)
	require.Equal(t, http.StatusCreated, response.StatusCode, body)
	require.NotNil(t, got.OriginShiftID)
	assert.Equal(t, int64(9223372036854775807), *got.OriginShiftID)
	assert.Equal(t, "09:00:00", got.StartTime, "HH:MM is widened to the capability wall clock")
	assert.Equal(t, testActorStaffID, got.ActorStaffID, "the actor comes from the acting admin's staff record")
	assert.Contains(t, body, `"id":"9223372036854775806"`, "bigint identifiers stay strings on the wire")
	assert.Contains(t, body, `"start_time":"09:00"`)

	response = route.do(t, http.MethodPost, "/", `{"staff_id":7,"date":"2026-07-07","start_time":"09:00","end_time":"15:00","origin_shift_id":42}`)
	require.Equal(t, http.StatusCreated, response.StatusCode, responseBody(t, response))
	require.NotNil(t, got.OriginShiftID)
	assert.Equal(t, int64(42), *got.OriginShiftID, "legacy numeric ids are still accepted")
}

func TestStaffShiftsRouteMoveMapsPayloadAndRequiresType(t *testing.T) {
	t.Parallel()
	var got workforce.MoveStaffShift
	called := 0
	fake := &fakePlanning{moveFn: func(_ context.Context, input workforce.MoveStaffShift) (workforce.PlannedShift, error) {
		called++
		got = input
		return workforce.PlannedShift{}, nil
	}}
	route := setupStaffShiftsRoute(t, fake)

	response := route.do(t, http.MethodPut, "/9/move", `{"source_staff_id":7,"target_staff_id":8,"date":"2026-07-07","start_time":"09:00","end_time":"15:00","break_minutes":20,"shift_type_id":5}`)
	require.Equal(t, http.StatusOK, response.StatusCode, responseBody(t, response))
	assert.EqualValues(t, 9, got.ShiftID)
	assert.EqualValues(t, 7, got.SourceStaffID)
	assert.EqualValues(t, 8, got.TargetStaffID)
	assert.Equal(t, "2026-07-07", got.Date)
	assert.Equal(t, "09:00:00", got.StartTime)
	assert.Equal(t, "15:00:00", got.EndTime)
	require.NotNil(t, got.ShiftTypeID)
	assert.EqualValues(t, 5, *got.ShiftTypeID)
	assert.Equal(t, testActorStaffID, got.ActorStaffID)
	require.NotNil(t, got.ActorAccountID)
	assert.EqualValues(t, 7, *got.ActorAccountID)

	response = route.do(t, http.MethodPut, "/9/move", `{"source_staff_id":7,"target_staff_id":8,"date":"2026-07-07","start_time":"09:00","end_time":"15:00","break_minutes":20}`)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode, responseBody(t, response))
	assert.Equal(t, 1, called, "a move without an explicit shift type never reaches the capability")
}

func TestStaffShiftsRouteCancellationPayloads(t *testing.T) {
	t.Parallel()
	var got workforce.CancelStaffShift
	called := 0
	fake := &fakePlanning{cancelFn: func(_ context.Context, input workforce.CancelStaffShift) (workforce.StaffShiftCancellation, error) {
		called++
		got = input
		return workforce.StaffShiftCancellation{Replacements: []workforce.PlannedShift{}}, nil
	}}
	route := setupStaffShiftsRoute(t, fake)

	// A full origin payload maps every field onto ApplyOriginEdits without loss.
	response := route.do(t, http.MethodPut, "/5/cancellation", `{"cancelled": true, "change_reason": "krank", "start_time": "09:00", "end_time": "14:00", "break_minutes": 30, "shift_type_id": 7}`)
	body := responseBody(t, response)
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	assert.True(t, got.ApplyOriginEdits)
	assert.Equal(t, 30, got.BreakMinutes)
	require.NotNil(t, got.ShiftTypeID)
	assert.EqualValues(t, 7, *got.ShiftTypeID)
	assert.Equal(t, testActorStaffID, got.ActorStaffID)
	assert.Contains(t, body, `"replacements":[]`, "an empty cover set stays an array")

	// Omitting all four origin fields preserves the stored origin values.
	response = route.do(t, http.MethodPut, "/5/cancellation", `{"cancelled": true, "change_reason": "krank"}`)
	require.Equal(t, http.StatusOK, response.StatusCode, responseBody(t, response))
	assert.False(t, got.ApplyOriginEdits)

	// An explicit null shift type alongside the full window is a legitimate
	// "clear the type" edit.
	response = route.do(t, http.MethodPut, "/5/cancellation", `{"cancelled": true, "start_time": "09:00", "end_time": "14:00", "break_minutes": 0, "shift_type_id": null}`)
	require.Equal(t, http.StatusOK, response.StatusCode, responseBody(t, response))
	assert.True(t, got.ApplyOriginEdits)
	assert.Nil(t, got.ShiftTypeID)
	before := called

	// A partial origin payload is rejected rather than silently zeroing the
	// omitted fields (#1841); so is an omitted cancelled flag.
	for name, payload := range map[string]string{
		"missing break_minutes": `{"cancelled": true, "start_time": "09:00", "end_time": "14:00", "shift_type_id": 7}`,
		"missing shift_type_id": `{"cancelled": true, "start_time": "09:00", "end_time": "14:00", "break_minutes": 30}`,
		"missing end_time":      `{"cancelled": true, "start_time": "09:00", "break_minutes": 30, "shift_type_id": 7}`,
		"only break_minutes":    `{"cancelled": true, "break_minutes": 30}`,
		"only shift_type_id":    `{"cancelled": true, "shift_type_id": 7}`,
		"null break_minutes":    `{"cancelled": true, "start_time": "09:00", "end_time": "14:00", "break_minutes": null, "shift_type_id": 7}`,
		"omitted cancelled":     `{"change_reason": "krank"}`,
		"null cancelled":        `{"cancelled": null}`,
	} {
		response := route.do(t, http.MethodPut, "/5/cancellation", payload)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode, name+": "+responseBody(t, response))
	}
	assert.Equal(t, before, called, "the capability must not be reached on a rejected payload")
}

func TestStaffShiftsRouteFailureEnvelope(t *testing.T) {
	t.Parallel()
	failure := workforce.ErrStaffShiftOverlap
	fake := &fakePlanning{
		cancelFn: func(context.Context, workforce.CancelStaffShift) (workforce.StaffShiftCancellation, error) {
			return workforce.StaffShiftCancellation{}, failure
		},
		getFn: func(context.Context, int64) (workforce.StaffShiftSeries, error) {
			return workforce.StaffShiftSeries{}, failure
		},
	}
	route := setupStaffShiftsRoute(t, fake)

	for _, test := range []struct {
		err    error
		status int
	}{
		{err: workforce.ErrStaffShiftOverlap, status: http.StatusConflict},
		{err: workforce.ErrStaffShiftConflict, status: http.StatusConflict},
		{err: workforce.ErrStaffShiftNotFound, status: http.StatusNotFound},
		{err: &workforce.InvalidStaffShiftError{Reason: "origin shift not found"}, status: http.StatusBadRequest},
		{err: workforce.ErrShiftTypeInactive, status: http.StatusBadRequest},
		{err: workforce.ErrShiftSeriesNotFound, status: http.StatusNotFound},
		{err: &workforce.InvalidShiftSeriesError{Reason: "weekdays must not repeat"}, status: http.StatusBadRequest},
		{err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	} {
		failure = test.err
		response := route.do(t, http.MethodPut, "/5/cancellation", `{"cancelled": true}`)
		body := responseBody(t, response)
		assert.Equal(t, test.status, response.StatusCode, test.err.Error()+": "+body)
		if test.status != http.StatusInternalServerError {
			assert.Contains(t, body, test.err.Error(), "the capability wording is the envelope message")
		}
		response = route.do(t, http.MethodGet, "/series/5", "")
		assert.Equal(t, test.status, response.StatusCode, test.err.Error()+": "+responseBody(t, response))
	}
}

func TestStaffShiftsRouteSeriesHandlers(t *testing.T) {
	t.Parallel()
	var created workforce.CreateStaffShiftSeries
	var split workforce.SplitStaffShiftSeries
	var endedID int64
	var endedFrom string
	fake := &fakePlanning{
		seriesFn: func(_ context.Context, input workforce.CreateStaffShiftSeries) (workforce.StaffShiftSeriesResult, error) {
			created = input
			return workforce.StaffShiftSeriesResult{SeriesID: 9223372036854775807, Created: 2, SkippedDates: []string{"2026-09-02"}}, nil
		},
		splitFn: func(_ context.Context, input workforce.SplitStaffShiftSeries) (workforce.StaffShiftSeriesResult, error) {
			split = input
			return workforce.StaffShiftSeriesResult{SeriesID: 12, OldSeriesID: 11, Created: 3, Deleted: 2}, nil
		},
		getFn: func(_ context.Context, id int64) (workforce.StaffShiftSeries, error) {
			return workforce.StaffShiftSeries{ID: id, StaffID: 5, Weekdays: []int{1, 3}, StartTime: "09:00:00", EndTime: "12:00:00", BreakMinutes: 15, CalendarPeriodID: 8, WeekPattern: 1, ValidFrom: "2026-09-01", ValidUntil: "2026-12-01"}, nil
		},
		endFn: func(_ context.Context, id int64, from string) (workforce.StaffShiftSeriesResult, error) {
			endedID, endedFrom = id, from
			return workforce.StaffShiftSeriesResult{SeriesID: id, Deleted: 4}, nil
		},
	}
	route := setupStaffShiftsRoute(t, fake)

	response := route.do(t, http.MethodPost, "/series", `{"staff_id": 5, "weekdays": [1, 3], "start_time": "09:00", "end_time": "12:00", "break_minutes": 15, "shift_type_id": null, "calendar_period_id": 8, "week_pattern": 1, "valid_from": "2026-09-01", "valid_until": "2026-12-01"}`)
	body := responseBody(t, response)
	require.Equal(t, http.StatusCreated, response.StatusCode, body)
	assert.Equal(t, []int{1, 3}, created.Weekdays)
	assert.Equal(t, "09:00:00", created.StartTime)
	assert.Equal(t, 1, created.WeekPattern)
	assert.Equal(t, "2026-12-01", created.ValidUntil)
	assert.Equal(t, testActorStaffID, created.ActorStaffID)
	assert.Contains(t, body, `"series_id":"9223372036854775807"`)
	assert.Contains(t, body, `"skipped_dates":["2026-09-02"]`)

	for name, payload := range map[string]string{
		"bad valid_from":       `{"staff_id": 5, "weekdays": [1], "start_time": "09:00", "end_time": "12:00", "calendar_period_id": 8, "valid_from": "September"}`,
		"bad time":             `{"staff_id": 5, "weekdays": [1], "start_time": "9", "end_time": "12:00", "calendar_period_id": 8, "valid_from": "2026-09-01"}`,
		"out-of-range weekday": `{"staff_id": 5, "weekdays": [65538], "start_time": "09:00", "end_time": "12:00", "calendar_period_id": 8, "valid_from": "2026-09-01"}`,
	} {
		response := route.do(t, http.MethodPost, "/series", payload)
		assert.Equal(t, http.StatusBadRequest, response.StatusCode, name+": "+responseBody(t, response))
	}

	// A split inherits every omitted field and keeps the lossless occurrence id.
	response = route.do(t, http.MethodPut, "/series/11/split", `{"effective_date": "2026-10-05", "start_time": "10:00", "end_time": "13:00", "break_minutes": 20, "occurrence_shift_id": "9007199254740993"}`)
	body = responseBody(t, response)
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	assert.Equal(t, int64(11), split.SeriesID)
	assert.Equal(t, "2026-10-05", split.EffectiveDate)
	assert.Equal(t, int64(9007199254740993), split.OccurrenceShiftID)
	assert.Nil(t, split.Weekdays, "omitted weekdays keep the predecessor's")
	assert.Nil(t, split.Notes)
	assert.Nil(t, split.WeekPattern)
	assert.False(t, split.ShiftTypeIDSet)
	assert.False(t, split.ValidUntilSet)
	assert.Contains(t, body, `"series_id":"12"`)
	assert.Contains(t, body, `"old_series_id":"11"`)

	response = route.do(t, http.MethodPut, "/series/11/split", `{"effective_date": "2026-10-05", "start_time": "10:00", "end_time": "13:00", "shift_type_id": null, "valid_until": null, "weekdays": [2]}`)
	require.Equal(t, http.StatusOK, response.StatusCode, responseBody(t, response))
	assert.True(t, split.ShiftTypeIDSet, "an explicit null clears the type")
	assert.True(t, split.ValidUntilSet, "an explicit null runs to the period end")
	assert.Equal(t, []int{2}, split.Weekdays)

	response = route.do(t, http.MethodPut, "/series/11/split", `{"effective_date": "next monday", "start_time": "10:00", "end_time": "13:00"}`)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)

	response = route.do(t, http.MethodGet, "/series/11", "")
	body = responseBody(t, response)
	require.Equal(t, http.StatusOK, response.StatusCode, body)
	assert.Contains(t, body, `"id":"11"`)
	assert.Contains(t, body, `"weekdays":[1,3]`)
	assert.Contains(t, body, `"start_time":"09:00"`)
	assert.Contains(t, body, `"valid_until":"2026-12-01"`)

	response = route.do(t, http.MethodDelete, "/series/11?from=2026-11-02", "")
	require.Equal(t, http.StatusOK, response.StatusCode, responseBody(t, response))
	assert.Equal(t, int64(11), endedID)
	assert.Equal(t, "2026-11-02", endedFrom)
	response = route.do(t, http.MethodDelete, "/series/11", "")
	assert.Equal(t, http.StatusBadRequest, response.StatusCode, "the end date is required")
}

func TestStaffShiftsRouteOverviewPermissionsAndWireContract(t *testing.T) {
	t.Parallel()
	reason := "krank"
	target := 1500
	fake := &fakePlanning{overviewFn: func(_ context.Context, from, to string) (workforce.StaffScheduleOverview, error) {
		return workforce.StaffScheduleOverview{
			From: from, To: to, DienstplanInUse: true, UsedWeeks: []string{"2070-11-03"},
			Staff:  []workforce.OverviewStaff{{ID: 3, FirstName: "Lea", LastName: "Leitung"}},
			Shifts: []workforce.PlannedShift{{StaffShift: workforce.StaffShift{ID: 9, StaffID: 3, Date: "2070-11-03", StartTime: "08:00:00", EndTime: "12:00:00"}, ShiftType: &workforce.ShiftTypeLabel{Name: "Betreuung", Color: "#83CD2D"}}},
			Assignments: []workforce.OverviewAssignment{{
				InstanceID: 4, StaffID: 3, Date: "2070-11-03", StartTime: "09:00:00", EndTime: "10:30:00", ActivityTitle: "Lesen",
				RoomID: 2, RoomName: "Raum 1", Status: "planned", IsAbsent: true, AbsenceReason: &reason, CoverageStatus: "uncovered",
				UncoveredIntervals: []workforce.CoverageInterval{{StartTime: "09:00:00", EndTime: "10:30:00"}},
			}},
			WeeklySummaries: []workforce.WeeklySummary{{StaffID: 3, WeekStart: "2070-11-03", PlannedMinutes: 240, TargetMinutes: &target}},
		}, nil
	}}
	route := setupStaffShiftsRoute(t, fake)
	path := "/overview?from=2070-11-03&to=2070-11-07"

	for name, granted := range map[string][]string{
		"shift only":    {permissions.TimeTrackingManage},
		"schedule only": {permissions.SchedulesRead},
		"without users": {permissions.TimeTrackingManage, permissions.SchedulesRead},
	} {
		request := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil, testutil.WithJWTBearer(route.claims(granted...)))
		assert.Equal(t, http.StatusForbidden, testutil.ExecuteRequest(route.router, request).Code, name)
	}
	request := testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil,
		testutil.WithJWTBearer(route.claims(permissions.TimeTrackingManage, permissions.SchedulesRead, permissions.UsersRead)))
	recorder := testutil.ExecuteRequest(route.router, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	assert.JSONEq(t, `"2070-11-03"`, string(envelope.Data["from"]))
	assert.JSONEq(t, `true`, string(envelope.Data["dienstplan_in_use"]))
	assert.JSONEq(t, `["2070-11-03"]`, string(envelope.Data["dienstplan_used_weeks"]))
	assert.JSONEq(t, `[{"id":3,"first_name":"Lea","last_name":"Leitung"}]`, string(envelope.Data["staff"]))
	assert.JSONEq(t, `[{"id":"9","staff_id":3,"date":"2070-11-03","start_time":"08:00","end_time":"12:00","break_minutes":0,"shift_type_name":"Betreuung","shift_type_color":"#83CD2D","detached":false,"cancelled":false}]`, string(envelope.Data["shifts"]))
	assert.JSONEq(t, `[{"instance_id":4,"staff_id":3,"date":"2070-11-03","start_time":"09:00","end_time":"10:30","activity_title":"Lesen","room_id":2,"room_name":"Raum 1","status":"planned","is_absent":true,"is_substitute":false,"absence_reason":"krank","coverage_status":"uncovered","coverage_reason":null,"uncovered_intervals":[{"start_time":"09:00","end_time":"10:30"}]}]`, string(envelope.Data["assignments"]))
	assert.JSONEq(t, `[{"staff_id":3,"week_start":"2070-11-03","planned_minutes":240,"target_minutes":1500,"delta_minutes":null}]`, string(envelope.Data["weekly_summaries"]))

	// Empty projections stay arrays, a bad range is a bad request, and an
	// unexpected failure hides its cause behind the stable message.
	fake.overviewFn = func(_ context.Context, from, to string) (workforce.StaffScheduleOverview, error) {
		return workforce.StaffScheduleOverview{From: from, To: to}, nil
	}
	recorder = testutil.ExecuteRequest(route.router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil,
		testutil.WithJWTBearer(route.claims(permissions.TimeTrackingManage, permissions.SchedulesRead, permissions.UsersRead))))
	require.Equal(t, http.StatusOK, recorder.Code)
	for _, key := range []string{`"dienstplan_used_weeks":[]`, `"staff":[]`, `"shifts":[]`, `"assignments":[]`, `"weekly_summaries":[]`} {
		assert.Contains(t, recorder.Body.String(), key)
	}
	recorder = testutil.ExecuteRequest(route.router, testutil.NewAuthenticatedRequest(t, http.MethodGet, "/overview?from=2070-11-03", nil,
		testutil.WithJWTBearer(route.claims(permissions.TimeTrackingManage, permissions.SchedulesRead, permissions.UsersRead))))
	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	const rawCause = "pq: relation does not exist"
	fake.overviewFn = func(context.Context, string, string) (workforce.StaffScheduleOverview, error) {
		return workforce.StaffScheduleOverview{}, errors.New(rawCause)
	}
	recorder = testutil.ExecuteRequest(route.router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil,
		testutil.WithJWTBearer(route.claims(permissions.TimeTrackingManage, permissions.SchedulesRead, permissions.UsersRead))))
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"error":"staff schedule overview could not be loaded"`)
	assert.NotContains(t, recorder.Body.String(), rawCause)
}

func TestStaffShiftsRouteExportGuardsTheInternalVariant(t *testing.T) {
	t.Parallel()
	var got workforce.PlanExportRequest
	fake := &fakePlanning{exportFn: func(_ context.Context, request workforce.PlanExportRequest) (workforce.PlanExportFile, error) {
		got = request
		if request.Variant == "internal" && !request.AllowInternal {
			return workforce.PlanExportFile{}, workforce.ErrPlanExportForbidden
		}
		return workforce.PlanExportFile{ContentType: "application/pdf", Filename: "dienstplan.pdf", Data: []byte("%PDF")}, nil
	}}
	route := setupStaffShiftsRoute(t, fake)
	body := `{"from":"2070-11-03","to":"2070-11-07","template":"wall","variant":"internal","format":"pdf"}`

	reader := route.claims(permissions.TimeTrackingManage, permissions.SchedulesRead, permissions.UsersRead)
	recorder := testutil.ExecuteRequest(route.router, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/export", json.RawMessage(body), testutil.WithJWTBearer(reader)))
	assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	assert.False(t, got.AllowInternal)

	manager := route.claims(permissions.TimeTrackingManage, permissions.SchedulesRead, permissions.UsersRead, permissions.SchedulesManage)
	recorder = testutil.ExecuteRequest(route.router, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/export", json.RawMessage(body), testutil.WithJWTBearer(manager)))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.True(t, got.AllowInternal)
	assert.Equal(t, "application/pdf", recorder.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="dienstplan.pdf"`, recorder.Header().Get("Content-Disposition"))
	assert.Equal(t, "%PDF", recorder.Body.String())

	recorder = testutil.ExecuteRequest(route.router, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/export", json.RawMessage(body), testutil.WithJWTBearer(route.claims(permissions.TimeTrackingManage))))
	assert.Equal(t, http.StatusForbidden, recorder.Code, "the export needs the same triple as the overview")
}
