package timetablehttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// dayTransitionSpontaneousStart is the owner's spontaneous-start preparation
// for the day-transition tests: the room exists and is free, the room lock
// succeeds, and the activity resolution returns activityID. onRoomExists and
// onResolve observe the two steps; resolveCalls counts the resolutions.
type dayTransitionSpontaneousStart struct {
	activityID   *int64
	onRoomExists func()
	onResolve    func()
	resolveCalls int
}

func (s *dayTransitionSpontaneousStart) data() timetable.TimetableDataCapability {
	return &fakeTimetableData{
		SpontaneousRoomExistsFn: func(context.Context, int64) (bool, error) {
			if s.onRoomExists != nil {
				s.onRoomExists()
			}
			return true, nil
		},
		LockSpontaneousStartRoomFn: func(context.Context, int64) error { return nil },
		SpontaneousRoomOccupiedFn:  func(context.Context, int64) (bool, error) { return false, nil },
		ResolveSpontaneousActivityFn: func(context.Context, string, *int64, int64) (*int64, error) {
			s.resolveCalls++
			if s.onResolve != nil {
				s.onResolve()
			}
			return s.activityID, nil
		},
	}
}

func TestOperationsCreateAndStartSpontaneousRechecksWorkdayAfterRequestValidation(t *testing.T) {
	t.Parallel()

	roomChecked := false
	spontaneous := &dayTransitionSpontaneousStart{onRoomExists: func() { roomChecked = true }}
	service := &fakeOperationsService{start: &timetable.StartedOperation{Status: timetable.InstanceStatusActive}}
	res := NewResource(Dependencies{
		TimetableData:     spontaneous.data(),
		OperationsService: service,
		People:            staffAccountPeople(220, 320),
		SettingsService:   &fakeOperationSettingsService{hasOverride: true, boolValue: true},
		Now: func() time.Time {
			if roomChecked {
				return time.Date(2026, time.May, 9, 0, 0, 0, 0, calendar.Berlin)
			}
			return time.Date(2026, time.May, 8, 23, 59, 59, 0, calendar.Berlin)
		},
	})
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	roomID := int64(70)
	rr := executeOperationRequest(t, router, http.MethodPost, "/spontaneous/start", map[string]any{
		"title":   "Freispiel",
		"room_id": roomID,
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.True(t, roomChecked)
	// The activity resolution is the step that finds or creates the activity
	// and its category; it must not be reached at all.
	assert.Zero(t, spontaneous.resolveCalls, "the weekend recheck must run before activity resolution")
	assert.Nil(t, service.lastSpontaneousInput, "a request crossing into Saturday must not mutate")
}

func TestOperationsCreateAndStartSpontaneousRechecksWorkdayBeforeInstanceCreation(t *testing.T) {
	t.Parallel()

	activityResolved := false
	activityGroupID := int64(71)
	spontaneous := &dayTransitionSpontaneousStart{
		activityID: &activityGroupID,
		onResolve:  func() { activityResolved = true },
	}
	service := &fakeOperationsService{start: &timetable.StartedOperation{Status: timetable.InstanceStatusActive}}
	res := NewResource(Dependencies{
		TimetableData:     spontaneous.data(),
		OperationsService: service,
		People:            staffAccountPeople(220, 320),
		SettingsService:   &fakeOperationSettingsService{hasOverride: true, boolValue: true},
		Now: func() time.Time {
			if activityResolved {
				return time.Date(2026, time.May, 9, 0, 0, 0, 0, calendar.Berlin)
			}
			return time.Date(2026, time.May, 8, 23, 59, 59, 0, calendar.Berlin)
		},
	})
	router := operationRouter(http.MethodPost, "/spontaneous/start", res.operationsCreateAndStartSpontaneous)

	req := httptest.NewRequest(http.MethodPost, "/spontaneous/start", strings.NewReader(`{"title":"Freispiel","room_id":70}`))
	req.Header.Set("Content-Type", "application/json")
	testutil.WithClaims(t, testutil.AdminTestClaims(120))(req)
	testutil.WithPermissions(permissions.UsersRead)(req)
	*req = *req.WithContext(tenant.WithRollbackMarker(req.Context()))
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.True(t, activityResolved)
	assert.Nil(t, service.lastSpontaneousInput, "a request crossing into Saturday during activity resolution must not create an instance")
	assert.True(t, tenant.RollbackRequested(req.Context()), "a rejected request after activity resolution must roll back metadata writes")
}
