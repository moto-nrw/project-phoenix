package timetablehttp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	timetablehttp "github.com/moto-ogs/project-phoenix/modules/timetable/http"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// These owner-handler measurements exclude outer transport, authentication and
// session liveness. SELECT-only participants/history budgets exclude BEGIN/COMMIT
// but include the role/tenant setup SELECT. MeasuredTenantRequest excludes the
// entire transaction frame for conflicts/staff pool.
func TestParticipantsRouteGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := timetablehttp.NewParticipantsGrowthFixture(t)
	counter := testpkg.CaptureQueries(t, s.DB)
	router := s.Router(s.Context)
	var small int
	for _, n := range []int{3, 8} {
		for len(s.StudentIDs) < n {
			child := testpkg.CreateTestStudent(t, s.DB, "Participant", fmt.Sprintf("Growth %d", len(s.StudentIDs)), "3a")
			testpkg.CreateTestInstanceStudent(t, s.DB, s.InstanceID, child.ID, timetable.SlotAttendanceExpected)
			s.StudentIDs = append(s.StudentIDs, child.ID)
		}
		counter.Reset()
		response := httptest.NewRecorder()
		testpkg.MountedQueryBudgetRouter("/api/timetable", router).ServeHTTP(response, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/instances/%d/participants", s.InstanceID), nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var body timetablehttp.InstanceParticipantsResponse
		decodeGrowthEnvelope(t, response, &body)
		require.Len(t, body.Participants, n)
		require.Len(t, body.Staff, 1)
		require.Equal(t, s.InstanceID, body.InstanceID)
		require.Equal(t, s.StaffID, body.Staff[0].StaffID)
		ids := make([]int64, 0, n)
		for _, child := range body.Participants {
			ids = append(ids, child.StudentID)
			require.NotEmpty(t, child.DisplayName)
		}
		require.ElementsMatch(t, s.StudentIDs, ids)
		require.Zero(t, counter.WriteRows(), "read request must not write")
		queries := counter.Measurement(counter.Operation("SELECT"))
		if n == 3 {
			small = len(queries.Statements)
		} else {
			require.Equal(t, small, len(queries.Statements))
		}
		t.Logf("participants growth N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.timetable.instance_participants.list", queries)
	}
	foreign := testpkg.NewTenantScope(t, s.DB)
	foreignRouter := s.Router(foreign.Context())
	response := httptest.NewRecorder()
	testpkg.MountedQueryBudgetRouter("/api/timetable", foreignRouter).ServeHTTP(response, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/instances/%d/participants", s.InstanceID), nil))
	require.Equal(t, http.StatusNotFound, response.Code, response.Body.String())
}

func TestStaffPoolAndConflictsRouteGrowthQueryBudgets(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := timetablehttp.NewConflictsGrowthFixture(t)
	target := testpkg.CreateTestActivityInstance(t, s.DB, s.Date, s.RoomID, testpkg.ActivityInstanceOpts{StartHHMM: "14:00", EndHHMM: "15:00"})
	router := s.Router(s.Context)
	counter := testpkg.CaptureQueries(t, s.DB)
	measured := testpkg.MeasuredTenantRequest(t, s.DB, testpkg.TenantIDFromContext(s.Context), counter, router)
	staffIDs, studentIDs := []string{}, []string{}
	small := map[string]int{}
	for _, n := range []int{3, 8} {
		for len(staffIDs) < n {
			staff := testpkg.CreateTestStaff(t, s.DB, "Pool", fmt.Sprintf("Growth %d", len(staffIDs)))
			child := testpkg.CreateTestStudent(t, s.DB, "Conflict", fmt.Sprintf("Growth %d", len(studentIDs)), "3a")
			testpkg.CreateTestStaffShift(t, s.DB, staff.ID, s.Date, testpkg.StaffShiftOpts{})
			testpkg.CreateTestInstanceStaff(t, s.DB, target.ID, staff.ID, testpkg.InstanceStaffOpts{})
			testpkg.CreateTestInstanceStudent(t, s.DB, target.ID, child.ID, timetable.SlotAttendanceExpected)
			staffIDs = append(staffIDs, fmt.Sprint(staff.ID))
			studentIDs = append(studentIDs, fmt.Sprint(child.ID))
		}
		for _, pool := range []bool{false, true} {
			path := fmt.Sprintf("/conflicts?date=%s&start_time=14:00&end_time=15:00&staff_ids=%s&student_ids=%s", s.Date, strings.Join(staffIDs, ","), strings.Join(studentIDs, ","))
			scenario := "api.timetable.conflicts.list"
			if pool {
				path = fmt.Sprintf("/instances/%d/staff-pool", target.ID)
				scenario = "api.timetable.staff_pool.list"
			}
			counter.Reset()
			response := httptest.NewRecorder()
			testpkg.MountedQueryBudgetRouter("/api/timetable", measured).ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			if pool {
				var body timetablehttp.StaffPoolResponse
				decodeGrowthEnvelope(t, response, &body)
				require.Len(t, body.Entries, n)
			} else {
				var body timetablehttp.PlannedConflictsResponse
				decodeGrowthEnvelope(t, response, &body)
				require.Len(t, body.Warnings, 2*n)
			}
			require.Zero(t, counter.WriteRows(), "read request must not write")
			queries := counter.Measurement(counter.Operation("SELECT"))
			if n == 3 {
				small[scenario] = len(queries.Statements)
			} else {
				require.Equal(t, small[scenario], len(queries.Statements), scenario)
			}
			t.Logf("%s growth N%d statements=%d", scenario, n, len(queries.Statements))
			if pool {
				querybudgets.AssertQueryBudget(t, "api.timetable.staff_pool.list", queries)
			} else {
				querybudgets.AssertQueryBudget(t, "api.timetable.conflicts.list", queries)
			}
		}
	}
	foreign := testpkg.NewTenantScope(t, s.DB)
	foreignRouter := s.Router(foreign.Context())
	foreignRequest := testpkg.MeasuredTenantRequest(t, s.DB, foreign.TenantID, counter, foreignRouter)
	response := httptest.NewRecorder()
	testpkg.MountedQueryBudgetRouter("/api/timetable", foreignRequest).ServeHTTP(response, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/conflicts?date=%s&start_time=14:00&end_time=15:00&staff_ids=%s&student_ids=%s", s.Date, strings.Join(staffIDs, ","), strings.Join(studentIDs, ",")), nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var foreignConflicts timetablehttp.PlannedConflictsResponse
	decodeGrowthEnvelope(t, response, &foreignConflicts)
	require.Empty(t, foreignConflicts.Warnings)
	response = httptest.NewRecorder()
	testpkg.MountedQueryBudgetRouter("/api/timetable", foreignRequest).ServeHTTP(response, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/instances/%d/staff-pool", target.ID), nil))
	require.Equal(t, http.StatusNotFound, response.Code, response.Body.String())
}

func TestStudentWeekPopulatedGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	s := timetablehttp.NewStudentDayGrowthFixture(t)
	counter := testpkg.CaptureQueries(t, s.DB)
	router := s.Router(s.Context)
	from := calendar.NewDate(2026, 4, 20)
	created, small := 0, 0
	for _, n := range []int{3, 8} {
		for created < n {
			date := from.AddDays(created)
			instance := testpkg.CreateTestActivityInstance(t, s.DB, date, s.RoomID, testpkg.ActivityInstanceOpts{ActivityGroupID: &s.ActivityID})
			testpkg.CreateTestInstanceStudent(t, s.DB, instance.ID, s.StudentID, timetable.SlotAttendanceExpected)
			testpkg.CreateTestArrivalException(t, s.DB, s.StudentID, date, s.StaffID, "13:00", "growth")
			testpkg.CreateTestPickupException(t, s.DB, s.StudentID, date, s.StaffID, "16:00", "growth")
			created++
		}
		counter.Reset()
		response := doGrowthGet(t, router, fmt.Sprintf("/student/%d/week?from=%s&to=%s", s.StudentID, from, from.AddDays(13)))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var body timetablehttp.StudentWeekResponse
		decodeGrowthEnvelope(t, response, &body)
		require.Len(t, body.Days, 14)
		var envelope struct {
			Data struct {
				Days []struct {
					Instances []json.RawMessage `json:"instances"`
				} `json:"days"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
		count := 0
		for _, day := range envelope.Data.Days {
			count += len(day.Instances)
		}
		require.Equal(t, n, count)
		require.Zero(t, counter.WriteRows(), "read request must not write")
		queries := counter.Measurement(counter.Operation("SELECT"))
		if n == 3 {
			small = len(queries.Statements)
		} else {
			require.Equal(t, small, len(queries.Statements))
		}
		t.Logf("student week growth N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.timetable.student_week.14d", queries)
	}
}

// History names are best effort per identity. Record the existing fan-out so
// a future batch contract must preserve partial names rather than hide it.
func TestDeviationHistoryPopulatedQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := timetablehttp.NewHistoryGrowthFixture(t)
	writeRouter := s.WriteRouter
	router := s.Router(s.Context)
	counter := testpkg.CaptureQueries(t, s.DB)
	date := calendar.NewDate(2027, 4, 19)
	created, small := 0, 0
	for _, n := range []int{3, 8} {
		for created < n {
			staff := testpkg.CreateTestStaff(t, s.DB, "History", fmt.Sprintf("Growth %d", created))
			instance := testpkg.CreateTestActivityInstance(t, s.DB, date, s.RoomID, testpkg.ActivityInstanceOpts{Title: fmt.Sprintf("History %d", created)})
			testpkg.CreateTestInstanceStaff(t, s.DB, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
			response := doGrowthDev(t, writeRouter, instance.ID, map[string]any{"substitutions": []map[string]any{{"absent_staff_id": staff.ID, "substitute_staff_id": s.StaffID, "reason": "krank"}}})
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			created++
		}
		counter.Reset()
		response := httptest.NewRecorder()
		testpkg.MountedQueryBudgetRouter("/api/timetable", router).ServeHTTP(response, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/deviations/history?date=%s&date_to=%s", date, date), nil))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var body timetablehttp.DeviationHistoryResponse
		decodeGrowthEnvelope(t, response, &body)
		require.Len(t, body.Events, n)
		for _, event := range body.Events {
			require.NotNil(t, event.SubjectStaffName)
			require.NotNil(t, event.RelatedStaffName)
			require.Contains(t, *event.SubjectStaffName, "History Growth")
			require.Equal(t, date.String(), event.OccurrenceDate)
			require.NotNil(t, event.Reason)
			require.Equal(t, "krank", *event.Reason)
		}
		require.Zero(t, counter.WriteRows(), "read request must not write")
		queries := counter.Measurement(counter.Operation("SELECT"))
		if n == 3 {
			small = len(queries.Statements)
		} else {
			t.Logf("history statement growth N3=%d N8=%d", small, len(queries.Statements))
		}
		require.Len(t, queries.Statements, n+4, "one membership per identity; unused Employment reads are removed under E1 (#5892)")
		require.Empty(t, counter.Selects("staff_employment_profiles"))
		t.Logf("deviation history N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.timetable.deviation_history.list", queries)
	}
	foreign := testpkg.NewTenantScope(t, s.DB)
	response := httptest.NewRecorder()
	testpkg.MountedQueryBudgetRouter("/api/timetable", s.Router(foreign.Context())).ServeHTTP(response, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/deviations/history?date=%s&date_to=%s", date, date), nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var empty timetablehttp.DeviationHistoryResponse
	decodeGrowthEnvelope(t, response, &empty)
	require.Empty(t, empty.Events)
}

func TestStudentDayPopulatedGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	s := timetablehttp.NewStudentDayGrowthFixture(t)
	counter := testpkg.CaptureQueries(t, s.DB)
	router := s.Router(s.Context)
	date := calendar.NewDate(2026, 4, 22)
	created, small := 0, 0
	for _, n := range []int{3, 8} {
		for created < n {
			activity := testpkg.CreateTestActivityGroup(t, s.DB, fmt.Sprintf("Day growth %d", created))
			instance := testpkg.CreateTestActivityInstance(t, s.DB, date, s.RoomID, testpkg.ActivityInstanceOpts{ActivityGroupID: &activity.ID})
			testpkg.CreateTestInstanceStudent(t, s.DB, instance.ID, s.StudentID, timetable.SlotAttendanceExpected)
			created++
		}
		counter.Reset()
		response := doGrowthGet(t, router, fmt.Sprintf("/student/%d/day?date=%s", s.StudentID, date))
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var body struct {
			StudentID int64  `json:"student_id"`
			Date      string `json:"date"`
			Instances []struct {
				ID int64 `json:"id"`
			} `json:"instances"`
		}
		decodeGrowthEnvelope(t, response, &body)
		require.Equal(t, s.StudentID, body.StudentID)
		require.Equal(t, date.String(), body.Date)
		require.Len(t, body.Instances, n)
		require.Zero(t, counter.WriteRows())
		queries := counter.Measurement(counter.Queries())
		if n == 3 {
			small = len(queries.Statements)
		} else {
			require.Equal(t, small, len(queries.Statements))
		}
		t.Logf("student day growth N%d statements=%d", n, len(queries.Statements))
		querybudgets.AssertQueryBudget(t, "api.timetable.student_day.populated", queries)
	}
	response := doGrowthGet(t, router, fmt.Sprintf("/student/%d/day?date=%s", s.StudentID, date.AddDays(1)))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var empty struct {
		Instances []json.RawMessage `json:"instances"`
	}
	decodeGrowthEnvelope(t, response, &empty)
	require.Empty(t, empty.Instances)
	foreign := testpkg.NewTenantScope(t, s.DB)
	response = doGrowthGet(t, s.Router(foreign.Context()), fmt.Sprintf("/student/%d/day?date=%s", s.StudentID, date))
	require.Equal(t, http.StatusNotFound, response.Code, response.Body.String())
}

func decodeGrowthEnvelope(t *testing.T, w *httptest.ResponseRecorder, target any) {
	t.Helper()
	var envelope struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope), "body=%s", w.Body.String())
	require.Equal(t, "success", envelope.Status, "body=%s", w.Body.String())
	require.NoError(t, json.Unmarshal(envelope.Data, target))
}
func doGrowthGet(t *testing.T, router chi.Router, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	testpkg.MountedQueryBudgetRouter("/api/timetable", router).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}
func doGrowthDev(t *testing.T, router chi.Router, instanceID int64, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/instances/%d/deviations", instanceID), bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	testpkg.MountedQueryBudgetRouter("/api/timetable", router).ServeHTTP(w, request)
	return w
}
