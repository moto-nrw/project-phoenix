package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lifecycleStoreStub struct {
	LifecycleStore
	group                                       LifecycleSession
	startErr, touchErr, countErr, supervisorErr error
	conflict                                    devicescan.ConflictInfoResponse
	conflictErr                                 error
	starts, touches                             int
}

func (s *lifecycleStoreStub) Start(context.Context, int64, devicescan.StartSessionCommand) (LifecycleSession, error) {
	s.starts++
	return s.group, s.startErr
}
func (s *lifecycleStoreStub) Current(context.Context, int64) (*LifecycleSession, error) {
	return &s.group, nil
}
func (s *lifecycleStoreStub) ReplaceSupervisors(context.Context, int64, []int64) (LifecycleSession, error) {
	return s.group, nil
}
func (s *lifecycleStoreStub) Supervisors(context.Context, int64) ([]LifecycleSupervisor, error) {
	return s.group.Supervisors, s.supervisorErr
}
func (s *lifecycleStoreStub) CountStudents(context.Context, int64) (int, error) { return 4, s.countErr }
func (s *lifecycleStoreStub) Touch(context.Context, int64) error                { s.touches++; return s.touchErr }
func (s *lifecycleStoreStub) Conflict(context.Context, int64, int64) (devicescan.ConflictInfoResponse, error) {
	return s.conflict, s.conflictErr
}

type lifecyclePeopleStub struct {
	ids []int64
	err error
}

func (p *lifecyclePeopleStub) StaffNames(_ context.Context, ids []int64) (map[int64]ports.Person, error) {
	p.ids = ids
	result := make(map[int64]ports.Person, len(ids))
	for _, id := range ids {
		result[id] = ports.Person{FirstName: "Test", LastName: "Supervisor"}
	}
	return result, p.err
}

type lifecycleHeartbeatStub struct {
	err   error
	calls int
}

func (p *lifecycleHeartbeatStub) PingDevice(context.Context, string) error { p.calls++; return p.err }

func lifecycleForTest(store *lifecycleStoreStub, people *lifecyclePeopleStub) *sessionLifecycle {
	return &sessionLifecycle{store: store, people: people, principals: fakePrincipals{device: testDevice()}, heartbeat: &lifecycleHeartbeatStub{}, clock: fakeClock{now: time.Now()}, logger: slog.Default()}
}

// These cases migrate the old HTTP helper tests to the command that owns the
// active-supervisor filtering and its one batch name lookup.
func TestReplaceSessionSupervisorsFiltersEndedAndInvalidStaff(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		supervisors []LifecycleSupervisor
		ids         []int64
	}{
		{"all active", []LifecycleSupervisor{{StaffID: 1}, {StaffID: 2}, {StaffID: 3}}, []int64{1, 2, 3}},
		{"some ended", []LifecycleSupervisor{{StaffID: 1}, {StaffID: 2, Ended: true}, {StaffID: 3}}, []int64{1, 3}},
		{"all ended", []LifecycleSupervisor{{StaffID: 1, Ended: true}, {StaffID: 2, Ended: true}}, nil},
		{"empty", nil, nil},
		{"invalid IDs", []LifecycleSupervisor{{StaffID: 1}, {StaffID: 0}, {StaffID: -1}, {StaffID: 2}}, []int64{1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &lifecycleStoreStub{group: LifecycleSession{ID: 77, Supervisors: tc.supervisors}}
			people := &lifecyclePeopleStub{}
			result, err := lifecycleForTest(store, people).ReplaceSessionSupervisors(t.Context(), 77, []int64{1})
			require.NoError(t, err)
			require.NotNil(t, result.Supervisors)
			assert.Equal(t, tc.ids, people.ids)
			assert.Len(t, result.Supervisors, len(tc.ids))
			for i, id := range tc.ids {
				assert.Equal(t, id, result.Supervisors[i].StaffID)
			}
		})
	}
}

func TestSessionSupervisorLookupFailureProducesEmptyJSONArray(t *testing.T) {
	t.Parallel()
	store := &lifecycleStoreStub{group: LifecycleSession{ID: 77, Supervisors: []LifecycleSupervisor{{StaffID: 303, Role: "supervisor"}}}}
	result, err := lifecycleForTest(store, &lifecyclePeopleStub{err: errors.New("database unavailable")}).ReplaceSessionSupervisors(t.Context(), 77, []int64{303})
	require.NoError(t, err)
	require.NotNil(t, result.Supervisors)
	assert.Empty(t, result.Supervisors)
}

func TestCurrentSessionKeepsBestEffortFailuresOptional(t *testing.T) {
	t.Parallel()
	errUnavailable := errors.New("unavailable")
	store := &lifecycleStoreStub{group: LifecycleSession{ID: 77, RoomID: 5, StartTime: time.Now()}, touchErr: errUnavailable, countErr: errUnavailable, supervisorErr: errUnavailable}
	service := lifecycleForTest(store, &lifecyclePeopleStub{})
	heartbeat := &lifecycleHeartbeatStub{err: errUnavailable}
	service.heartbeat = heartbeat
	result, err := service.CurrentSession(t.Context())
	require.NoError(t, err)
	assert.True(t, result.IsActive)
	assert.Equal(t, int64(77), *result.ActiveGroupID)
	assert.Nil(t, result.ActiveStudents)
	assert.Nil(t, result.Supervisors)
	assert.Equal(t, 1, heartbeat.calls)
	assert.Equal(t, 1, store.touches)
}

func TestStartSessionRechecksConflictWithoutMaskingLookupFailure(t *testing.T) {
	t.Parallel()
	store := &lifecycleStoreStub{startErr: &SessionStartConflict{Cause: ports.ErrConflict}, conflict: devicescan.ConflictInfoResponse{HasConflict: true, ConflictMessage: "already active"}}
	service := lifecycleForTest(store, &lifecyclePeopleStub{})
	result, err := service.StartSession(t.Context(), devicescan.StartSessionCommand{ActivityID: 4, SupervisorIDs: []int64{1}})
	require.NoError(t, err)
	assert.Equal(t, "conflict", result.Status)
	require.NotNil(t, result.ConflictInfo)
	assert.Equal(t, "already active", result.Message)
	store.conflictErr = errors.New("lookup failed")
	_, err = service.StartSession(t.Context(), devicescan.StartSessionCommand{ActivityID: 4, SupervisorIDs: []int64{1}})
	failure, ok := devicescan.IsFailure(err)
	require.True(t, ok)
	assert.Equal(t, devicescan.FailureConflict, failure.Kind)
}

func TestSessionStartWithoutDeviceOrSupervisorsDoesNotWrite(t *testing.T) {
	t.Parallel()
	store := &lifecycleStoreStub{}
	service := lifecycleForTest(store, &lifecyclePeopleStub{})
	_, err := service.StartSession(t.Context(), devicescan.StartSessionCommand{ActivityID: 4})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one supervisor ID is required")
	service.principals = fakePrincipals{}
	_, err = service.StartSession(t.Context(), devicescan.StartSessionCommand{ActivityID: 4, SupervisorIDs: []int64{1}})
	assert.ErrorIs(t, err, devicescan.ErrDeviceUnauthorized)
	assert.Zero(t, store.starts)
}
