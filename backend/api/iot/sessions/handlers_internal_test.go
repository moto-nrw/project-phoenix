package sessions

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/iot"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// endSessionActiveServiceStub answers the two calls the session-end handler
// makes and records when the session end ran, so the ordering against the
// timetable bridge is observable.
type endSessionActiveServiceStub struct {
	activeSvc.Service
	session *active.Group
	endErr  error
	record  func(string)
}

func (s *endSessionActiveServiceStub) GetDeviceCurrentSession(context.Context, int64) (*active.Group, error) {
	return s.session, nil
}

func (s *endSessionActiveServiceStub) EndActivitySession(context.Context, int64) error {
	s.record("session")
	return s.endErr
}

func endSessionRequest(t *testing.T) *http.Request {
	t.Helper()
	dev := &iot.Device{}
	dev.ID = 9
	req := httptest.NewRequest(http.MethodPost, "/end", nil)
	return req.WithContext(context.WithValue(req.Context(), device.CtxDevice, dev))
}

// The timetable half of the close runs BEFORE the session end (#1747 review).
// EndActivitySession emits its checkout and activity-ended SSE eagerly, so
// running it first would announce a close that a failing bridge then rolls
// back — kiosks and dashboards would show a session the database still has
// running.
func TestEndActivitySessionCompletesTimetableBeforeEndingSession(t *testing.T) {
	newResource := func(t *testing.T, bridgeErr error, order *[]string) *Resource {
		t.Helper()
		activeGroupID := int64(66)
		inst := &scheduleModel.ActivityInstance{ActiveGroupID: &activeGroupID}
		inst.ID = 77
		repo := &mirrorInstanceRepoStub{
			findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
				return inst, nil
			},
			completeActive: func(context.Context, []int64, time.Time) (int64, error) {
				*order = append(*order, "timetable")
				return 1, bridgeErr
			},
		}
		session := &active.Group{StartTime: time.Now()}
		session.ID = activeGroupID
		return &Resource{
			ActiveService: &endSessionActiveServiceStub{
				session: session,
				record:  func(step string) { *order = append(*order, step) },
			},
			TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
				ActivityInstanceRepo: repo,
			}),
			TimetableBridge: mirrorBridge(repo),
		}
	}

	t.Run("happy path closes the timetable first", func(t *testing.T) {
		var order []string
		rr := httptest.NewRecorder()

		newResource(t, nil, &order).endActivitySession(rr, endSessionRequest(t))

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, []string{"timetable", "session"}, order)
	})

	t.Run("a failing bridge stops before anything is announced", func(t *testing.T) {
		var order []string
		rr := httptest.NewRecorder()

		newResource(t, errors.New("bridge down"), &order).endActivitySession(rr, endSessionRequest(t))

		require.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.Equal(t, []string{"timetable"}, order,
			"the session must not be ended — its SSE fires before the transaction commits")
	})

	// The tenant middleware rolls back on its own only for 5xx. A 4xx from the
	// session end after the bridge already completed the instance would commit a
	// completed timetable block next to a session that is still open (#1747
	// review), so that path has to ask for the rollback itself.
	t.Run("a 4xx after the bridge asks for a rollback", func(t *testing.T) {
		var order []string
		rs := newResource(t, nil, &order)
		rs.ActiveService.(*endSessionActiveServiceStub).endErr = &activeSvc.ActiveError{
			Op:  "EndActivitySession",
			Err: activeSvc.ErrActiveGroupAlreadyEnded,
		}

		req := endSessionRequest(t)
		req = req.WithContext(tenant.WithRollbackMarker(req.Context()))
		rr := httptest.NewRecorder()

		rs.endActivitySession(rr, req)

		require.Less(t, rr.Code, http.StatusInternalServerError,
			"already-ended maps to a 4xx, which the middleware would otherwise commit")
		assert.Equal(t, []string{"timetable", "session"}, order)
		assert.True(t, tenant.RollbackRequested(req.Context()),
			"the completed timetable instance must not survive a failed session end")
	})
}
