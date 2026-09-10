package sessions

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
)

// endSessionActiveServiceStub answers the one read the session-end handler
// still makes: which session runs on the calling device.
type endSessionActiveServiceStub struct {
	devicescan.SessionLifecycle
	session devicescan.SessionToEnd
	err     error
}

func (s *endSessionActiveServiceStub) SessionToEnd(context.Context) (devicescan.SessionToEnd, error) {
	return s.session, s.err
}

// sessionEndStub is the workflow facade: it records the group it was asked to
// close and answers with a fixed result or error.
type sessionEndStub struct {
	ended  []int64
	result sessionend.Result
	err    error
}

func (s *sessionEndStub) EndSession(_ context.Context, activeGroupID int64) (sessionend.Result, error) {
	s.ended = append(s.ended, activeGroupID)
	return s.result, s.err
}

func endSessionRequest(t *testing.T) *testpkg.HTTPRequest {
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
	session := devicescan.SessionToEnd{StartTime: startedAt}
	session.ID = 66
	workflow := &sessionEndStub{result: sessionend.Result{ActiveGroupID: 66, EndedAt: endedAt, StudentsCheckedOut: 2}}
	rs := &Resource{
		Lifecycle:  &endSessionActiveServiceStub{session: session},
		runtime:    testRuntime(),
		SessionEnd: workflow,
	}
	req := endSessionRequest(t)
	rr := httptest.NewRecorder()

	rs.endActivitySession(rr, req)

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

	cases := map[string]struct {
		err    error
		status int
	}{
		"already ended": {err: sessionend.ErrSessionAlreadyEnded, status: 400},
		"not found":     {err: sessionend.ErrSessionNotFound, status: 404},
		"owner failure": {err: errors.New("attendance finalization failed"), status: 500},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			session := devicescan.SessionToEnd{StartTime: time.Now()}
			session.ID = 66
			rs := &Resource{
				Lifecycle:  &endSessionActiveServiceStub{session: session},
				runtime:    testRuntime(),
				SessionEnd: &sessionEndStub{err: tc.err},
			}
			req := endSessionRequest(t)
			rr := httptest.NewRecorder()

			rs.endActivitySession(rr, req)

			assert.Equal(t, tc.status, rr.Code, rr.Body.String())
			assert.True(t, testutil.RollbackRequested(req.Context()), "a failed close must not commit")
		})
	}
}

// A device without a running session never reaches the workflow.
func TestEndActivitySessionWithoutSessionSkipsTheWorkflow(t *testing.T) {
	t.Parallel()

	workflow := &sessionEndStub{}
	rs := &Resource{
		Lifecycle:  &endSessionActiveServiceStub{err: devicescan.InvalidRequest("no active session to end")},
		runtime:    testRuntime(),
		SessionEnd: workflow,
	}
	rr := httptest.NewRecorder()

	rs.endActivitySession(rr, endSessionRequest(t))

	assert.Equal(t, 400, rr.Code, rr.Body.String())
	assert.Empty(t, workflow.ended)
}
