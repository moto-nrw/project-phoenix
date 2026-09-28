package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestAutoStart(t *testing.T, deps InstanceAutoStartDependencies) timetable.InstanceAutoStart {
	t.Helper()
	svc, err := NewInstanceAutoStart(deps)
	require.NoError(t, err)
	return svc
}

func TestAutoStart_RunForTenant_StartsOnlyDueStaffedConflictFreeInstances(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 13, 30, 0, 0, time.Local)
	repo := &autoStartInstanceRepo{instances: []*scheduleModel.ActivityInstance{
		autoStartInstance(101, scheduleModel.InstanceStatusPlanned, 14, 0, 15, 0),
		autoStartInstance(102, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
		autoStartInstance(103, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
		autoStartInstance(104, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
		autoStartInstance(105, scheduleModel.InstanceStatusPlanned, 12, 0, 13, 0),
		autoStartInstance(106, scheduleModel.InstanceStatusActive, 13, 0, 14, 0),
	}}
	staffRepo := &autoStartStaffRepo{counts: map[int64]int{
		102: 1,
		104: 1,
	}}
	starter := &autoStartInstanceStarter{}
	svc := newTestAutoStart(t, InstanceAutoStartDependencies{
		InstanceRepo:      repo,
		InstanceStaffRepo: staffRepo,
		Lifecycle:         starter,
		Rooms:             &autoStartRoomRepo{},
		Conflicts: startConflictsFunc(func(_ context.Context, subject timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error) {
			if subject.InstanceID == 104 {
				return []timetable.InstanceConflictWarning{{Kind: timetable.ConflictKindStaff, ResourceID: subject.RoomID, CanOverride: true}}, nil
			}
			return nil, nil
		}),
		Logger: slog.Default(),
	})

	result, err := svc.RunForTenant(context.Background(), now)

	require.NoError(t, err)
	assert.Equal(t, 6, result.Checked)
	assert.Equal(t, 1, result.Started)
	assert.Equal(t, 1, result.SkippedBeforeWindow)
	assert.Equal(t, 1, result.SkippedAfterWindow)
	assert.Equal(t, 1, result.SkippedNoStaff)
	assert.Equal(t, 1, result.SkippedConflict)
	assert.Equal(t, 1, result.SkippedNonPlanned)
	assert.Equal(t, []int64{102}, starter.startedIDs)
	assert.Equal(t, []int64{0}, starter.startedByStaffIDs, "auto-start must not claim a human starter")
	assert.Equal(t, []int64{101, 102, 103, 104, 105}, staffRepo.requestedIDs)
}

func TestAutoStart_RunForTenant_ReportsConflictReadFailureAndRetries(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 20, 13, 30, 0, 0, time.Local)
	injected := errors.New("presence lookup failed")
	readErr := injected
	starter := &autoStartInstanceStarter{}
	svc := newTestAutoStart(t, InstanceAutoStartDependencies{
		InstanceRepo: &autoStartInstanceRepo{instances: []*scheduleModel.ActivityInstance{
			autoStartInstance(201, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
		}},
		InstanceStaffRepo: &autoStartStaffRepo{counts: map[int64]int{201: 1}},
		Lifecycle:         starter,
		Rooms:             &autoStartRoomRepo{},
		Conflicts: startConflictsFunc(func(_ context.Context, subject timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error) {
			return nil, readErr
		}),
		Logger: slog.Default(),
	})
	result, err := svc.RunForTenant(context.Background(), now)
	require.ErrorIs(t, err, injected)
	assert.Equal(t, 1, result.Failed)
	assert.Zero(t, result.SkippedConflict)
	assert.Zero(t, result.Started)
	assert.Empty(t, starter.startedIDs)
	readErr = nil
	result, err = svc.RunForTenant(context.Background(), now)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Started)
	assert.Equal(t, []int64{201}, starter.startedIDs)
}

func TestAutoStart_RunForTenant_ReturnsStartError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 13, 30, 0, 0, time.Local)
	startErr := errors.New("start failed")
	repo := &autoStartInstanceRepo{instances: []*scheduleModel.ActivityInstance{
		autoStartInstance(201, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
	}}
	staffRepo := &autoStartStaffRepo{counts: map[int64]int{201: 1}}
	starter := &autoStartInstanceStarter{err: startErr}
	svc := newTestAutoStart(t, InstanceAutoStartDependencies{
		InstanceRepo:      repo,
		InstanceStaffRepo: staffRepo,
		Lifecycle:         starter,
		Rooms:             &autoStartRoomRepo{},
		Conflicts: startConflictsFunc(func(_ context.Context, subject timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error) {
			return nil, nil
		}),
		Logger: slog.Default(),
	})

	result, err := svc.RunForTenant(context.Background(), now)

	require.Error(t, err)
	assert.ErrorIs(t, err, startErr)
	assert.Equal(t, 1, result.Checked)
	assert.Equal(t, 1, result.Failed)
	assert.Equal(t, 0, result.Started)
}

// A block that Start reports as concurrently moved (ErrInstanceMoved) is a
// benign skip, not a batch abort: the move is already committed and the next
// scheduler tick re-reads and starts the block on its real day. The rest of the
// batch must still start (#1840).
func TestAutoStart_RunForTenant_SkipsMovedAndContinues(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 13, 30, 0, 0, time.Local)
	repo := &autoStartInstanceRepo{instances: []*scheduleModel.ActivityInstance{
		autoStartInstance(321, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
		autoStartInstance(322, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0),
	}}
	staffRepo := &autoStartStaffRepo{counts: map[int64]int{321: 1, 322: 1}}
	starter := &autoStartInstanceStarter{errByID: map[int64]error{
		321: fmt.Errorf("wrapped: %w", timetable.ErrInstanceMoved),
	}}
	svc := newTestAutoStart(t, InstanceAutoStartDependencies{
		InstanceRepo:      repo,
		InstanceStaffRepo: staffRepo,
		Lifecycle:         starter,
		Rooms:             &autoStartRoomRepo{},
		Conflicts: startConflictsFunc(func(_ context.Context, subject timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error) {
			return nil, nil
		}),
		Logger: slog.Default(),
	})

	result, err := svc.RunForTenant(context.Background(), now)

	require.NoError(t, err)
	assert.Equal(t, 2, result.Checked)
	assert.Equal(t, 1, result.SkippedMoved)
	assert.Equal(t, 1, result.Started)
	assert.Zero(t, result.Failed)
	assert.Equal(t, []int64{322}, starter.startedIDs)
}

// The Schulhof is a regular plannable room since #2161: its planned blocks
// are conflict-checked and auto-started exactly like any other room's.
func TestAutoStart_RunForTenant_StartsSchulhofLikeAnyRoom(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 13, 30, 0, 0, time.Local)
	schulhof := autoStartInstance(311, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0)
	schulhof.RoomID = 401
	normal := autoStartInstance(312, scheduleModel.InstanceStatusPlanned, 13, 0, 14, 0)
	normal.RoomID = 402
	conflictChecks := make([]int64, 0, 2)
	starter := &autoStartInstanceStarter{}
	svc := newTestAutoStart(t, InstanceAutoStartDependencies{
		InstanceRepo:      &autoStartInstanceRepo{instances: []*scheduleModel.ActivityInstance{schulhof, normal}},
		InstanceStaffRepo: &autoStartStaffRepo{counts: map[int64]int{311: 1, 312: 1}},
		Lifecycle:         starter,
		Rooms: &autoStartRoomRepo{rooms: map[int64]string{
			401: constants.SchulhofRoomName,
			402: "Lernraum",
		}},
		Conflicts: startConflictsFunc(func(_ context.Context, subject timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error) {
			conflictChecks = append(conflictChecks, subject.InstanceID)
			return nil, nil
		}),
		Logger: slog.Default(),
	})

	result, err := svc.RunForTenant(context.Background(), now)

	require.NoError(t, err)
	assert.Equal(t, 2, result.Started)
	assert.Zero(t, result.Failed)
	assert.Equal(t, []int64{311, 312}, conflictChecks)
	assert.Equal(t, []int64{311, 312}, starter.startedIDs)
}

// startConflictsFunc adapts a function to the Timetable owner's start check.
type startConflictsFunc func(context.Context, timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error)

func (f startConflictsFunc) DetectStartConflicts(ctx context.Context, subject timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error) {
	return f(ctx, subject)
}

func autoStartInstance(id int64, status string, startHour, startMinute, endHour, endMinute int) *scheduleModel.ActivityInstance {
	inst := &scheduleModel.ActivityInstance{
		Date:      scheduleModel.NewDate(2026, 4, 20),
		Title:     "Auto-start test",
		StartTime: time.Date(1, 1, 1, startHour, startMinute, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, endHour, endMinute, 0, 0, time.UTC),
		RoomID:    301,
		Status:    status,
	}
	inst.ID = id
	return inst
}

// autoStartInstanceRepo answers the reads of the auto start and the auto
// end; every other repository method is unused.
type autoStartInstanceRepo struct {
	scheduleModel.ActivityInstanceRepository
	instances      []*scheduleModel.ActivityInstance
	findStatusByID map[int64]string
}

func (r *autoStartInstanceRepo) FindByID(_ context.Context, rawID any) (*scheduleModel.ActivityInstance, error) {
	id, ok := rawID.(int64)
	if !ok {
		return nil, nil
	}
	for _, instance := range r.instances {
		if instance.ID != id {
			continue
		}
		copy := *instance
		if status, found := r.findStatusByID[id]; found {
			copy.Status = status
		}
		return &copy, nil
	}
	return nil, nil
}

func (r *autoStartInstanceRepo) List(context.Context, *ActivityInstanceQueryOptions) ([]*scheduleModel.ActivityInstance, error) {
	return r.instances, nil
}

func (r *autoStartInstanceRepo) FindByTenantAndDate(context.Context, scheduleModel.Date) ([]*scheduleModel.ActivityInstance, error) {
	return r.instances, nil
}

// autoStartRoomRepo names the rooms of the batch; a room without a
// configured name is a "Lernraum".
type autoStartRoomRepo struct {
	LifecycleRooms
	rooms map[int64]string
	err   error
}

func (r *autoStartRoomRepo) RoomNamesByID(_ context.Context, ids []int64) (map[int64]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	names := make(map[int64]string, len(ids))
	for _, id := range ids {
		if name, ok := r.rooms[id]; ok {
			names[id] = name
			continue
		}
		names[id] = "Lernraum"
	}
	return names, nil
}

type autoStartStaffRepo struct {
	scheduleModel.InstanceStaffRepository
	counts       map[int64]int
	requestedIDs []int64
}

func (r *autoStartStaffRepo) CountNonAbsentByInstanceIDs(_ context.Context, ids []int64) (map[int64]int, error) {
	r.requestedIDs = append(r.requestedIDs, ids...)
	return r.counts, nil
}

// autoStartInstanceStarter records the lifecycle starts; the other
// transitions are unused by the auto start.
type autoStartInstanceStarter struct {
	timetable.InstanceLifecycle
	startedIDs        []int64
	startedByStaffIDs []int64
	err               error
	errByID           map[int64]error
}

func (s *autoStartInstanceStarter) Start(_ context.Context, instanceID, startedByStaffID int64) (*timetable.StartInstanceResult, error) {
	if err := s.errByID[instanceID]; err != nil {
		return nil, err
	}
	if s.err != nil {
		return nil, s.err
	}
	s.startedIDs = append(s.startedIDs, instanceID)
	s.startedByStaffIDs = append(s.startedByStaffIDs, startedByStaffID)
	return &timetable.StartInstanceResult{Instance: &timetable.LifecycleInstance{}, ActiveGroupID: 401}, nil
}
