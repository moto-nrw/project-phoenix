package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sessionsAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/sessions"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
)

// iotEndSessionActiveServiceStub answers the one read the session-end handler
// still makes: which session runs on the calling device.
type iotEndSessionActiveServiceStub struct {
	iotTestSessionLifecycle
	session iotTestSessionToEnd
	err     error
}

func (s *iotEndSessionActiveServiceStub) SessionToEnd(context.Context) (iotTestSessionToEnd, error) {
	return s.session, s.err
}

// iotEndSessionWorkflowStub is the workflow facade: it records the group it was asked to
// close and answers with a fixed result or error.
type iotEndSessionWorkflowStub struct {
	ended  []int64
	result iotTestSessionEndResult
	err    error
}

func (s *iotEndSessionWorkflowStub) EndSession(_ context.Context, activeGroupID int64) (iotTestSessionEndResult, error) {
	s.ended = append(s.ended, activeGroupID)
	return s.result, s.err
}

func iotEndSessionRequest(t *testing.T) *testpkg.HTTPRequest {
	t.Helper()
	req := httptest.NewRequest("POST", "/end", nil)
	testutil.WithDeviceIdentity(9, "end-device")(req)
	return req.WithContext(testutil.WithRollbackMarker(req.Context()))
}

// The handler resolves the device's session and hands exactly that session to
// the one facade; nothing else about the close is decided here (#2697).
func TestEndActivitySessionDelegatesToTheSessionEndWorkflow(t *testing.T) {
	t.Parallel()

	startedAt := time.Now().Add(-90 * time.Minute)
	endedAt := time.Now()
	session := iotTestSessionToEnd{StartTime: startedAt}
	session.ID = 66
	workflow := &iotEndSessionWorkflowStub{result: iotTestSessionEndResult{ActiveGroupID: 66, EndedAt: endedAt, StudentsCheckedOut: 2}}
	rs := sessionsAPI.NewResource(&iotEndSessionActiveServiceStub{session: session}, BindIoTSessionEnd(workflow), iotSessionEndTestRuntime())
	req := iotEndSessionRequest(t)
	rr := httptest.NewRecorder()

	rs.Router().ServeHTTP(rr, req)

	require.Equal(t, 200, rr.Code, rr.Body.String())
	assert.Equal(t, []int64{66}, workflow.ended)
	assert.False(t, testutil.RollbackRequested(req.Context()))
	body := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok, "response carries the session summary: %v", body)
	assert.Equal(t, float64(66), data["active_group_id"])
	assert.Equal(t, "ended", data["status"])
	assert.Equal(t, endedAt.Sub(startedAt).String(), data["duration"], "the duration ends at the workflow's close instant")
}

// Every workflow failure asks the tenant middleware for a rollback, also the
// ones that map to a 4xx: the workflow joined the request transaction, so a
// committed partial close would otherwise survive (#1747 review).
func TestEndActivitySessionMapsWorkflowErrorsAndRollsBack(t *testing.T) {
	t.Parallel()

	notFound, alreadyEnded, conflict := iotSessionEndTestFailures()
	cases := map[string]struct {
		err    error
		status int
	}{
		"already ended":  {err: alreadyEnded, status: 400},
		"cohort changed": {err: conflict, status: 409},
		"not found":      {err: notFound, status: 404},
		"owner failure":  {err: errors.New("attendance finalization failed"), status: 500},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			session := iotTestSessionToEnd{StartTime: time.Now()}
			session.ID = 66
			rs := sessionsAPI.NewResource(&iotEndSessionActiveServiceStub{session: session}, BindIoTSessionEnd(&iotEndSessionWorkflowStub{err: tc.err}), iotSessionEndTestRuntime())
			req := iotEndSessionRequest(t)
			rr := httptest.NewRecorder()

			rs.Router().ServeHTTP(rr, req)

			assert.Equal(t, tc.status, rr.Code, rr.Body.String())
			if tc.status == http.StatusConflict {
				assert.Contains(t, rr.Body.String(), `"code":"general.business_rejection"`)
			}
			assert.True(t, testutil.RollbackRequested(req.Context()), "a failed close must not commit")
		})
	}
}

// A device without a running session never reaches the workflow.
func TestEndActivitySessionWithoutSessionSkipsTheWorkflow(t *testing.T) {
	t.Parallel()

	workflow := &iotEndSessionWorkflowStub{}
	rs := sessionsAPI.NewResource(&iotEndSessionActiveServiceStub{err: iotSessionEndTestInvalidRequest("no active session to end")}, BindIoTSessionEnd(workflow), iotSessionEndTestRuntime())
	rr := httptest.NewRecorder()

	rs.Router().ServeHTTP(rr, iotEndSessionRequest(t))

	assert.Equal(t, 400, rr.Code, rr.Body.String())
	assert.Empty(t, workflow.ended)
}

func iotSessionEndTestRuntime() sessionsAPI.Runtime {
	return sessionsAPI.Runtime{
		ParseID:       testpkg.ParseHTTPIDParam,
		Authenticated: func(ctx context.Context) bool { _, _, ok := testutil.DeviceIdentity(ctx); return ok },
		Success:       testutil.RespondSuccess,
		Failure: func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest, status int, err error, message string) {
			testutil.RespondCoded(w, r, status, "", err, nil, message)
		},
		Conflict: func(w testpkg.HTTPResponseWriter, r *testpkg.HTTPRequest, message string, info iotTestSessionConflict) {
			render.Status(r, http.StatusConflict)
			render.JSON(w, r, map[string]any{"status": "error", "error": message, "details": info})
		},
		MarkRollback: testutil.MarkRollback,
	}
}
