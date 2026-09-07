package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	worktimemodelsHTTP "github.com/moto-nrw/project-phoenix/api/work-time-models"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// init seeds the JWT viper defaults before any test constructs a router; CI
// runs without a .env file.
func init() { testutil.SeedTestJWTConfig() }

// capabilityStub answers the two write calls /api/work-time-models makes on
// PUT and records what the handler asked for.
type capabilityStub struct {
	workforce.Capability
	updateErr  error
	refreshErr error
	updated    bool
	refreshed  bool
}

func (s *capabilityStub) UpdateWorkTimeModel(_ context.Context, input workforce.UpdateWorkTimeModel) (workforce.WorkTimeModel, error) {
	s.updated = true
	if s.updateErr != nil {
		return workforce.WorkTimeModel{}, s.updateErr
	}
	return workforce.WorkTimeModel{ID: input.ID, Name: input.Name, RotationLength: input.RotationLength, RotationAnchorDate: input.RotationAnchorDate}, nil
}

func (s *capabilityStub) RefreshAssignedStaffSchedules(context.Context, int64) error {
	s.refreshed = true
	return s.refreshErr
}

// updateRequest drives PUT /{id} straight at the handler, past the
// authentication and transaction middleware the composition root supplies.
func updateRequest(t *testing.T, capability workforce.Capability, notify func(context.Context)) *httptest.ResponseRecorder {
	t.Helper()
	resource := worktimemodelsHTTP.NewResource(capability, worktimemodelsHTTP.Runtime{
		Protected: func(router chi.Router, routes func(chi.Router, worktimemodelsHTTP.Middleware)) {
			routes(router, passthrough)
		},
		Permission: func(string) worktimemodelsHTTP.Middleware { return passthrough },
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

func passthrough(next http.Handler) http.Handler { return next }

// A template edit rewrites the schedule snapshots of every assigned staff
// member. The time-account views may only be invalidated once that second
// write succeeded, or clients would refetch a half-applied Soll.
func TestUpdateNotifiesTimeTrackingOnlyAfterTheAssignedRefreshSucceeds(t *testing.T) {
	t.Parallel()

	t.Run("notifies after both writes", func(t *testing.T) {
		t.Parallel()
		capability := &capabilityStub{}
		notified := false
		recorder := updateRequest(t, capability, func(context.Context) { notified = true })

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.True(t, capability.updated)
		assert.True(t, capability.refreshed)
		assert.True(t, notified)
	})

	t.Run("stays silent when the refresh fails", func(t *testing.T) {
		t.Parallel()
		capability := &capabilityStub{refreshErr: errors.New("refresh failed")}
		notified := false
		recorder := updateRequest(t, capability, func(context.Context) { notified = true })

		assert.Equal(t, http.StatusInternalServerError, recorder.Code)
		assert.True(t, capability.refreshed)
		assert.False(t, notified)
	})
}

// The production runtime is the only place the permission is chosen, so the
// rejection is asserted against the composed router rather than a stub.
func TestRouterRejectsUsersRead(t *testing.T) {
	t.Parallel()

	resource := worktimemodelsHTTP.NewResource(&capabilityStub{}, runtime(nil, func(context.Context) {}))
	router := resource.Router()
	claims := testutil.DefaultTestClaims()
	claims.Roles = []string{"user"}
	claims.Permissions = []string{"users:read"}
	claims.IsAdmin = false
	token := testutil.MintTestJWT(t, claims)

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
		request := testutil.NewAuthenticatedRequest(t, test.method, test.path, nil, testutil.WithJWTBearer(token))
		recorder := testutil.ExecuteRequest(router, request)

		require.Equal(t, http.StatusForbidden, recorder.Code)
	}
}
