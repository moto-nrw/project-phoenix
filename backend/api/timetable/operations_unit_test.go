package timetable

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationsPlannedNow(t *testing.T) {
	service := &fakeOperationsService{
		planned: []scheduleSvc.OperationPlannedInstance{{ID: 220, Title: "Lernzeit"}},
	}
	res := NewResource(Dependencies{OperationsService: service})
	router := operationRouter(http.MethodGet, "/planned-now", res.operationsPlannedNow)

	rr := executeOperationRequest(router, http.MethodGet, "/planned-now?date=2026-05-10", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(120), service.lastAccountID)
	assert.True(t, service.lastIsAdmin)
	assert.Equal(t, "2026-05-10", service.lastDate.Format(dateLayout))
	assert.Contains(t, rr.Body.String(), `"instances"`)
}

func TestOperationsPlannedNowValidationAndWiring(t *testing.T) {
	res := NewResource(Dependencies{})
	router := operationRouter(http.MethodGet, "/planned-now", res.operationsPlannedNow)
	rr := executeOperationRequest(router, http.MethodGet, "/planned-now", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	res = NewResource(Dependencies{OperationsService: &fakeOperationsService{}})
	router = operationRouter(http.MethodGet, "/planned-now", res.operationsPlannedNow)
	rr = executeOperationRequest(router, http.MethodGet, "/planned-now?date=bad", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperationsInstanceEndpoints(t *testing.T) {
	service := &fakeOperationsService{
		roster: &scheduleSvc.OperationRoster{Instance: scheduleSvc.OperationRosterInstance{ID: 230}},
		start: &scheduleSvc.StartInstanceResult{
			Instance:      &schedule.ActivityInstance{Status: schedule.InstanceStatusActive},
			ActiveGroupID: 330,
		},
		complete: &schedule.ActivityInstance{Status: schedule.InstanceStatusCompleted},
	}
	service.start.Instance.ID = 231
	service.complete.ID = 232
	res := NewResource(Dependencies{OperationsService: service})

	cases := []struct {
		name   string
		method string
		path   string
		fn     http.HandlerFunc
	}{
		{"roster", http.MethodGet, "/instances/230/roster", res.operationsRoster},
		{"start", http.MethodPost, "/instances/231/start", res.operationsStart},
		{"complete", http.MethodPost, "/instances/232/complete", res.operationsComplete},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := operationRouter(tc.method, "/instances/{id}/"+lastPathSegment(tc.path), tc.fn)

			rr := executeOperationRequest(router, tc.method, tc.path, nil)

			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
	assert.Equal(t, int64(232), service.lastInstanceID)
}

func TestOperationsRosterByActiveGroup(t *testing.T) {
	service := &fakeOperationsService{
		roster: &scheduleSvc.OperationRoster{Instance: scheduleSvc.OperationRosterInstance{ID: 240}},
	}
	res := NewResource(Dependencies{OperationsService: service})
	router := operationRouter(http.MethodGet, "/active-groups/{id}/roster", res.operationsRosterByActiveGroup)

	rr := executeOperationRequest(router, http.MethodGet, "/active-groups/340/roster", nil)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(340), service.lastActiveGroupID)

	nilRes := NewResource(Dependencies{})
	nilRouter := operationRouter(http.MethodGet, "/active-groups/{id}/roster", nilRes.operationsRosterByActiveGroup)
	rr = executeOperationRequest(nilRouter, http.MethodGet, "/active-groups/340/roster", nil)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	rr = executeOperationRequest(router, http.MethodGet, "/active-groups/nope/roster", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOperationsStudentEndpoints(t *testing.T) {
	service := &fakeOperationsService{
		roster: &scheduleSvc.OperationRoster{Instance: scheduleSvc.OperationRosterInstance{ID: 250}},
	}
	res := NewResource(Dependencies{OperationsService: service})

	checkInRouter := operationRouter(http.MethodPost, "/instances/{id}/students/{student_id}/check-in", res.operationsCheckInStudent)
	rr := executeOperationRequest(checkInRouter, http.MethodPost, "/instances/250/students/350/check-in", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(250), service.lastInstanceID)
	assert.Equal(t, int64(350), service.lastStudentID)

	checkOutRouter := operationRouter(http.MethodPost, "/instances/{id}/students/{student_id}/check-out", res.operationsCheckOutStudent)
	rr = executeOperationRequest(checkOutRouter, http.MethodPost, "/instances/251/students/351/check-out", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(251), service.lastInstanceID)
	assert.Equal(t, int64(351), service.lastStudentID)

	rr = executeOperationRequest(checkInRouter, http.MethodPost, "/instances/bad/students/350/check-in", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	errorRouter := operationRouter(
		http.MethodPost,
		"/instances/{id}/students/{student_id}/check-in",
		NewResource(Dependencies{OperationsService: &fakeOperationsService{err: scheduleSvc.ErrTimetableOperationConflict}}).operationsCheckInStudent,
	)
	rr = executeOperationRequest(errorRouter, http.MethodPost, "/instances/250/students/350/check-in", nil)
	assert.Equal(t, http.StatusConflict, rr.Code)

	errorRouter = operationRouter(
		http.MethodPost,
		"/instances/{id}/students/{student_id}/check-out",
		NewResource(Dependencies{OperationsService: &fakeOperationsService{err: scheduleSvc.ErrTimetableOperationNotFound}}).operationsCheckOutStudent,
	)
	rr = executeOperationRequest(errorRouter, http.MethodPost, "/instances/250/students/350/check-out", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestOperationsPatchAttendanceValidatesAndDelegates(t *testing.T) {
	status := schedule.AttendanceStatusAbsent
	service := &fakeOperationsService{
		patchRow: &scheduleSvc.OperationRosterRow{StudentID: 360, Status: schedule.AttendanceStatusAbsent},
	}
	res := NewResource(Dependencies{OperationsService: service})
	router := operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)

	rr := executeOperationRequest(router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{
		"status": status,
		"note":   "abgemeldet",
	})

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, int64(260), service.lastInstanceID)
	assert.Equal(t, int64(360), service.lastStudentID)
	require.NotNil(t, service.lastPatch.Status)
	assert.Equal(t, status, *service.lastPatch.Status)
}

func TestOperationsPatchAttendanceRejectsBadRequests(t *testing.T) {
	nilService := NewResource(Dependencies{})
	nilRouter := operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", nilService.operationsPatchAttendance)
	rr := executeOperationRequest(nilRouter, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{"status": "present"})
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	res := NewResource(Dependencies{OperationsService: &fakeOperationsService{}})
	router := operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)

	rr = executeOperationRequest(router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{})
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(router, http.MethodPatch, "/instances/260/students/360/attendance", "{bad json")
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	res = NewResource(Dependencies{OperationsService: &fakeOperationsService{
		err: &scheduleSvc.TimetableAttendanceValidationError{
			Fields: []schedule.AttendancePatchFieldError{{Field: "substatus", Reason: "cannot be set when status is expected"}},
		},
	}})
	router = operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)
	rr = executeOperationRequest(router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{"substatus": "sick"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "substatus")

	res = NewResource(Dependencies{
		OperationsService: &fakeOperationsService{err: scheduleSvc.ErrTimetableOperationForbidden},
	})
	router = operationRouter(http.MethodPatch, "/instances/{id}/students/{student_id}/attendance", res.operationsPatchAttendance)
	rr = executeOperationRequest(router, http.MethodPatch, "/instances/260/students/360/attendance", map[string]any{"status": "absent"})
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestOperationsIDParsingAndErrorMapping(t *testing.T) {
	res := NewResource(Dependencies{OperationsService: &fakeOperationsService{err: scheduleSvc.ErrTimetableOperationForbidden}})
	router := operationRouter(http.MethodGet, "/instances/{id}/roster", res.operationsRoster)
	rr := executeOperationRequest(router, http.MethodGet, "/instances/abc/roster", nil)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = executeOperationRequest(router, http.MethodGet, "/instances/260/roster", nil)
	assert.Equal(t, http.StatusForbidden, rr.Code)

	errorCases := []struct {
		err  error
		code int
	}{
		{scheduleSvc.ErrTimetableOperationNotFound, http.StatusNotFound},
		{scheduleSvc.ErrTimetableOperationConflict, http.StatusConflict},
		{scheduleSvc.ErrInvalidInstanceTransition, http.StatusConflict},
		{scheduleSvc.ErrInstanceNotFound, http.StatusNotFound},
		{activeSvc.ErrStudentAlreadyActive, http.StatusConflict},
		{activeSvc.ErrRoomConflict, http.StatusConflict},
		{activeSvc.ErrStudentNotFound, http.StatusNotFound},
		{activeSvc.ErrVisitNotFound, http.StatusNotFound},
		{activeSvc.ErrInvalidData, http.StatusBadRequest},
		{errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tc := range errorCases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			res := NewResource(Dependencies{OperationsService: &fakeOperationsService{err: tc.err}})
			router := operationRouter(http.MethodGet, "/instances/{id}/roster", res.operationsRoster)

			rr := executeOperationRequest(router, http.MethodGet, "/instances/260/roster", nil)

			assert.Equal(t, tc.code, rr.Code)
		})
	}
}

type fakeOperationsService struct {
	planned  []scheduleSvc.OperationPlannedInstance
	roster   *scheduleSvc.OperationRoster
	start    *scheduleSvc.StartInstanceResult
	complete *schedule.ActivityInstance
	patchRow *scheduleSvc.OperationRosterRow
	err      error

	lastAccountID     int64
	lastIsAdmin       bool
	lastDate          time.Time
	lastInstanceID    int64
	lastActiveGroupID int64
	lastStudentID     int64
	lastPatch         schedule.AttendanceFieldPatch
}

func (s *fakeOperationsService) PlannedNow(_ context.Context, accountID int64, isAdmin bool, date time.Time, _ time.Time) ([]scheduleSvc.OperationPlannedInstance, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastDate = date
	return s.planned, s.err
}

func (s *fakeOperationsService) Start(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*scheduleSvc.StartInstanceResult, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.start, s.err
}

func (s *fakeOperationsService) Complete(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*schedule.ActivityInstance, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.complete, s.err
}

func (s *fakeOperationsService) Roster(_ context.Context, accountID int64, isAdmin bool, instanceID int64) (*scheduleSvc.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	return s.roster, s.err
}

func (s *fakeOperationsService) RosterByActiveGroup(_ context.Context, accountID int64, isAdmin bool, activeGroupID int64) (*scheduleSvc.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastActiveGroupID = activeGroupID
	return s.roster, s.err
}

func (s *fakeOperationsService) CheckInStudent(_ context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*scheduleSvc.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	s.lastStudentID = studentID
	return s.roster, s.err
}

func (s *fakeOperationsService) CheckOutStudent(_ context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*scheduleSvc.OperationRoster, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	s.lastStudentID = studentID
	return s.roster, s.err
}

func (s *fakeOperationsService) PatchAttendance(_ context.Context, accountID int64, isAdmin bool, instanceID, studentID int64, patch schedule.AttendanceFieldPatch) (*scheduleSvc.OperationRosterRow, error) {
	s.lastAccountID = accountID
	s.lastIsAdmin = isAdmin
	s.lastInstanceID = instanceID
	s.lastStudentID = studentID
	s.lastPatch = patch
	return s.patchRow, s.err
}

func operationRouter(method, path string, handler http.HandlerFunc) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	switch method {
	case http.MethodGet:
		router.Get(path, handler)
	case http.MethodPost:
		router.Post(path, handler)
	case http.MethodPatch:
		router.Patch(path, handler)
	}
	return router
}

func executeOperationRequest(router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	switch v := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(v))
	default:
		raw, _ := json.Marshal(v)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	testutil.WithClaims(testutil.AdminTestClaims(120))(req)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func lastPathSegment(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
