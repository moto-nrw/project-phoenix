package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	worktimemodelsHTTP "github.com/moto-nrw/project-phoenix/modules/workforce/http/worktimemodels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// init seeds the JWT viper defaults before any test constructs a router; CI
// runs without a .env file.
func init() { routetest.SeedTestJWTConfig() }

// workTimeModelsCapabilityStub answers the one write call /api/work-time-models makes on
// PUT. The capability rewrites the assigned staff schedules inside that same
// call, so the handler has nothing left to sequence.
type workTimeModelsCapabilityStub struct {
	workforce.Capability
	updateErr error
	updated   bool
	findErr   error
	deleteErr error
}

func (s *workTimeModelsCapabilityStub) FindWorkTimeModel(_ context.Context, id int64) (workforce.WorkTimeModel, error) {
	if s.findErr != nil {
		return workforce.WorkTimeModel{}, s.findErr
	}
	return workforce.WorkTimeModel{ID: id, Name: "Vollzeit", RotationLength: 1, RotationAnchorDate: "2026-07-01"}, nil
}

func (s *workTimeModelsCapabilityStub) DeleteWorkTimeModel(context.Context, int64) error {
	return s.deleteErr
}

func (s *workTimeModelsCapabilityStub) UpdateWorkTimeModel(_ context.Context, input workforce.UpdateWorkTimeModel) (workforce.WorkTimeModel, error) {
	s.updated = true
	if s.updateErr != nil {
		return workforce.WorkTimeModel{}, s.updateErr
	}
	return workforce.WorkTimeModel{ID: input.ID, Name: input.Name, RotationLength: input.RotationLength, RotationAnchorDate: input.RotationAnchorDate}, nil
}

// updateWorkTimeModelsRequest drives PUT /{id} straight at the handler, past the
// authentication and transaction middleware the composition root supplies.
func updateWorkTimeModelsRequest(t *testing.T, capability workforce.Capability, notify func(context.Context)) *httptest.ResponseRecorder {
	t.Helper()
	resource := worktimemodelsHTTP.NewResource(capability, worktimemodelsHTTP.Runtime{
		Protected: func(router chi.Router, routes func(chi.Router, worktimemodelsHTTP.Middleware)) {
			routes(router, workTimeModelsPassthrough)
		},
		Permission: func(string) worktimemodelsHTTP.Middleware { return workTimeModelsPassthrough },
		ParseID:    func(*http.Request) (int64, error) { return 7, nil },
		Success: func(w http.ResponseWriter, _ *http.Request, status int, _ any, _ string) {
			w.WriteHeader(status)
		},
		Failure: func(w http.ResponseWriter, _ *http.Request, _ worktimemodelsHTTP.FailureKind, _ error) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		Manage:        "time_tracking:manage",
		NotifyChanged: notify,
	})
	body := `{"name":"Vollzeit","rotation_length":1,"rotation_anchor_date":"2026-07-01","entries":[]}`
	request := httptest.NewRequest(http.MethodPut, "/7", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	resource.Router().ServeHTTP(recorder, request)
	return recorder
}

func workTimeModelsPassthrough(next http.Handler) http.Handler { return next }

// A template edit also rewrites the schedule snapshots of every assigned
// staff member. The time-account views may only be invalidated once that
// whole write committed, or clients would refetch a half-applied Soll.
func TestUpdateNotifiesTimeTrackingOnlyAfterTheWriteSucceeds(t *testing.T) {
	t.Parallel()

	t.Run("notifies after the write", func(t *testing.T) {
		t.Parallel()
		capability := &workTimeModelsCapabilityStub{}
		notified := false
		recorder := updateWorkTimeModelsRequest(t, capability, func(context.Context) { notified = true })

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.True(t, capability.updated)
		assert.True(t, notified)
	})

	t.Run("stays silent when the write fails", func(t *testing.T) {
		t.Parallel()
		capability := &workTimeModelsCapabilityStub{updateErr: errors.New("assigned refresh failed")}
		notified := false
		recorder := updateWorkTimeModelsRequest(t, capability, func(context.Context) { notified = true })

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.True(t, capability.updated)
		assert.False(t, notified)
	})
}

// The production runtime is the only place the permission is chosen, so the
// rejection is asserted against the composed router rather than a stub.
func TestRouterRejectsUsersRead(t *testing.T) {
	t.Parallel()

	resource := worktimemodelsHTTP.NewResource(&workTimeModelsCapabilityStub{}, workTimeModelsRuntime(func(context.Context) {}))
	router := resource.Router()
	claims := routetest.DefaultTestClaims()
	claims.Roles = []string{"user"}
	claims.Permissions = []string{"users:read"}
	claims.IsAdmin = false
	token := routetest.MintTestJWT(t, claims)

	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/"},
		{method: http.MethodGet, path: "/123"},
		{method: http.MethodPost, path: "/"},
		{method: http.MethodPut, path: "/123"},
		{method: http.MethodDelete, path: "/123"},
	} {
		request := routetest.NewAuthenticatedRequest(t, test.method, test.path, nil, routetest.WithJWTBearer(token))
		recorder := routetest.ExecuteRequest(router, request)

		require.Equal(t, http.StatusForbidden, recorder.Code)
	}
}

// A stale model ID is an expected refusal (#4599): the production failure
// renderer answers 404 with the class code, not a server error.
func TestUpdateOfAnUnknownWorkTimeModelAnswersNotFound(t *testing.T) {
	t.Parallel()

	runtime := workTimeModelsRuntime(func(context.Context) {})
	runtime.Protected = func(router chi.Router, routes func(chi.Router, worktimemodelsHTTP.Middleware)) {
		routes(router, workTimeModelsPassthrough)
	}
	runtime.Permission = func(string) worktimemodelsHTTP.Middleware { return workTimeModelsPassthrough }
	capability := &workTimeModelsCapabilityStub{updateErr: workforce.ErrWorkTimeModelNotFound}
	resource := worktimemodelsHTTP.NewResource(capability, runtime)
	body := `{"name":"Vollzeit","rotation_length":1,"rotation_anchor_date":"2026-07-01","entries":[]}`
	request := httptest.NewRequest(http.MethodPut, "/7", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	resource.Router().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	assert.Equal(t, routetest.ClassCode(http.StatusNotFound), routetest.ParseJSONResponse(t, recorder.Body.Bytes())["code"])
}

// Reading and deleting a model keep their refusals apart (#4601): an unknown
// model is 404, one still assigned to staff 409, and a failed read or write
// is a server error rather than a masked "not found".
func TestGetAndDeleteSeparateNotFoundConflictAndServerErrors(t *testing.T) {
	t.Parallel()

	runtime := workTimeModelsRuntime(func(context.Context) {})
	runtime.Protected = func(router chi.Router, routes func(chi.Router, worktimemodelsHTTP.Middleware)) {
		routes(router, workTimeModelsPassthrough)
	}
	runtime.Permission = func(string) worktimemodelsHTTP.Middleware { return workTimeModelsPassthrough }
	dbFailure := errors.New("connection reset")

	for _, test := range []struct {
		name       string
		method     string
		capability *workTimeModelsCapabilityStub
		status     int
	}{
		{name: "get unknown", method: http.MethodGet, capability: &workTimeModelsCapabilityStub{findErr: workforce.ErrWorkTimeModelNotFound}, status: http.StatusNotFound},
		{name: "get read failure", method: http.MethodGet, capability: &workTimeModelsCapabilityStub{findErr: dbFailure}, status: http.StatusInternalServerError},
		{name: "get found", method: http.MethodGet, capability: &workTimeModelsCapabilityStub{}, status: http.StatusOK},
		{name: "delete unknown", method: http.MethodDelete, capability: &workTimeModelsCapabilityStub{deleteErr: workforce.ErrWorkTimeModelNotFound}, status: http.StatusNotFound},
		{name: "delete assigned", method: http.MethodDelete, capability: &workTimeModelsCapabilityStub{deleteErr: fmt.Errorf("delete: %w", workforce.ErrWorkTimeModelAssigned)}, status: http.StatusConflict},
		{name: "delete write failure", method: http.MethodDelete, capability: &workTimeModelsCapabilityStub{deleteErr: dbFailure}, status: http.StatusInternalServerError},
		{name: "delete succeeds", method: http.MethodDelete, capability: &workTimeModelsCapabilityStub{}, status: http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			resource := worktimemodelsHTTP.NewResource(test.capability, runtime)
			request := httptest.NewRequest(test.method, "/7", nil)
			recorder := httptest.NewRecorder()
			resource.Router().ServeHTTP(recorder, request)

			require.Equal(t, test.status, recorder.Code, recorder.Body.String())
			if test.status >= http.StatusBadRequest {
				assert.Equal(t, routetest.ClassCode(test.status), routetest.ParseJSONResponse(t, recorder.Body.Bytes())["code"])
			}
		})
	}
}
