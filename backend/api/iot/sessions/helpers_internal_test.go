package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mirrorInstanceRepoStub struct {
	scheduleModel.ActivityInstanceRepository
	findByActiveGroupID func(context.Context, int64) (*scheduleModel.ActivityInstance, error)
	create              func(context.Context, *scheduleModel.ActivityInstance) error
	markCompleted       func(context.Context, int64, time.Time) error
	completeActive      func(context.Context, []int64, time.Time) (int64, error)
}

func (r *mirrorInstanceRepoStub) FindByActiveGroupID(ctx context.Context, activeGroupID int64) (*scheduleModel.ActivityInstance, error) {
	return r.findByActiveGroupID(ctx, activeGroupID)
}

func (r *mirrorInstanceRepoStub) Create(ctx context.Context, inst *scheduleModel.ActivityInstance) error {
	return r.create(ctx, inst)
}

func (r *mirrorInstanceRepoStub) MarkCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error {
	return r.markCompleted(ctx, instanceID, completedAt)
}

func (r *mirrorInstanceRepoStub) CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	if r.completeActive != nil {
		return r.completeActive(ctx, activeGroupIDs, completedAt)
	}
	return 1, nil
}

// mirrorInstanceStudentRepoStub is the attendance half the bridge finalizes
// before an instance may be stamped completed. The mirrored-completion tests
// only care THAT the bridge runs, so the writes are no-ops.
type mirrorInstanceStudentRepoStub struct {
	scheduleModel.InstanceStudentRepository
}

func (r *mirrorInstanceStudentRepoStub) MarkNotScheduled(context.Context, []scheduleModel.StudentInstanceRef) error {
	return nil
}

func (r *mirrorInstanceStudentRepoStub) MarkExpectedAbsentByActiveGroupIDs(context.Context, []int64, time.Time, []scheduleModel.StudentInstanceRef) error {
	return nil
}

// mirrorBridge builds the completion bridge the IoT session-end path goes
// through (#1747): completing the instance without finalizing its attendance
// first would leave every expected row behind.
func mirrorBridge(instances scheduleModel.ActivityInstanceRepository) *scheduleSvc.TimetableBridgeService {
	return scheduleSvc.NewTimetableBridgeService(scheduleSvc.TimetableBridgeDependencies{
		Instances:        instances,
		InstanceStudents: &mirrorInstanceStudentRepoStub{},
	})
}

type mirrorInstanceStaffRepoStub struct {
	scheduleModel.InstanceStaffRepository
	created []*scheduleModel.InstanceStaff
	err     error
}

func (r *mirrorInstanceStaffRepoStub) Create(_ context.Context, row *scheduleModel.InstanceStaff) error {
	if r.err != nil {
		return r.err
	}
	r.created = append(r.created, row)
	return nil
}

type mirrorActivitiesServiceStub struct {
	activitiesSvc.ActivityService
	group *activityModels.Group
	err   error
}

func (s *mirrorActivitiesServiceStub) GetGroup(context.Context, int64) (*activityModels.Group, error) {
	return s.group, s.err
}

func TestBroadcastMirroredInstanceFiresAfterCommit(t *testing.T) {
	bc := testpkg.NewRecordingBroadcaster()
	rs := &Resource{Broadcaster: bc}
	ctx, drain := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 42))
	activeGroupID := int64(321)
	inst := &scheduleModel.ActivityInstance{
		Date:          timezone.NewDate(2026, 5, 11),
		StartTime:     time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		ActiveGroupID: &activeGroupID,
	}
	inst.ID = 123

	rs.broadcastMirroredInstance(ctx, realtime.EventInstanceStarted, inst, &active.Group{RoomID: 77})

	assert.Empty(t, bc.CallsByMethod("tenant"), "mirrored timetable SSE must wait for commit")

	drain()

	calls := bc.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
	assert.Equal(t, realtime.EventInstanceStarted, calls[0].Event.Type)
	assert.Equal(t, "321", calls[0].Event.ActiveGroupID)
	requireData := calls[0].Event.Data
	assert.NotNil(t, requireData.InstanceID)
	assert.Equal(t, "123", *requireData.InstanceID)
}

func TestMirrorSessionToTimetableCreatesInstanceStaffAndBroadcasts(t *testing.T) {
	ctx, drain := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 42))
	groupID := int64(44)
	roomID := int64(55)
	startedAt := time.Date(2026, 5, 12, 21, 45, 0, 0, time.UTC)
	var createdInstance *scheduleModel.ActivityInstance
	instanceRepo := &mirrorInstanceRepoStub{
		findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
			return nil, nil
		},
		create: func(_ context.Context, inst *scheduleModel.ActivityInstance) error {
			inst.ID = 77
			createdInstance = inst
			return nil
		},
	}
	staffRepo := &mirrorInstanceStaffRepoStub{}
	bc := testpkg.NewRecordingBroadcaster()
	rs := &Resource{
		TimetableData:     scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{ActivityInstanceRepo: instanceRepo, InstanceStaffRepo: staffRepo}),
		Broadcaster:       bc,
		ActivitiesService: &mirrorActivitiesServiceStub{group: &activityModels.Group{Name: "Werkstatt"}},
	}

	rs.mirrorSessionToTimetable(ctx, &active.Group{
		GroupID:   &groupID,
		StartTime: startedAt,
		RoomID:    roomID,
	}, []int64{101, 0, 101, 202})

	require.NotNil(t, createdInstance)
	assert.Equal(t, int64(77), createdInstance.ID)
	assert.Equal(t, &groupID, createdInstance.ActivityGroupID)
	assert.Equal(t, "Werkstatt", createdInstance.Title)
	assert.Equal(t, roomID, createdInstance.RoomID)
	assert.Equal(t, int64(101), *createdInstance.CreatedBy)
	assert.Equal(t, int64(42), createdInstance.GetTenantID())
	assert.Equal(t, "23:30", createdInstance.StartTime.Format("15:04"))
	assert.Equal(t, "23:59", createdInstance.EndTime.Format("15:04"))
	require.Len(t, staffRepo.created, 2)
	assert.Equal(t, int64(101), staffRepo.created[0].StaffID)
	assert.Equal(t, int64(202), staffRepo.created[1].StaffID)
	assert.Empty(t, bc.CallsByMethod("tenant"))

	drain()

	calls := bc.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, realtime.EventInstanceStarted, calls[0].Event.Type)
}

func TestMirrorHelperFallbacks(t *testing.T) {
	rs := &Resource{}

	assert.Equal(t, "RFID-Aktivität", rs.timetableTitleForActivity(context.Background(), nil))
	assert.Equal(t, time.Date(2000, 1, 1, 9, 5, 0, 0, time.UTC), clockFromMinutes(9*60+5))
	require.NotNil(t, firstPositiveID([]int64{-1, 0, 88}))
	assert.Equal(t, int64(88), *firstPositiveID([]int64{-1, 0, 88}))
	assert.Nil(t, firstPositiveID([]int64{-1, 0}))
	assert.Equal(t, []int64{88, 99}, sliceutil.UniquePositive([]int64{0, 88, 88, -1, 99}))
}

func TestMirrorSessionToTimetableSkipsWhenAlreadyMirroredOrLookupFails(t *testing.T) {
	groupID := int64(44)
	activeGroup := &active.Group{GroupID: &groupID, StartTime: time.Now()}
	activeGroup.ID = 66

	t.Run("already mirrored", func(t *testing.T) {
		createCalled := false
		rs := &Resource{
			TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
				ActivityInstanceRepo: &mirrorInstanceRepoStub{
					findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
						return &scheduleModel.ActivityInstance{}, nil
					},
					create: func(context.Context, *scheduleModel.ActivityInstance) error {
						createCalled = true
						return nil
					},
				},
				InstanceStaffRepo: &mirrorInstanceStaffRepoStub{},
			}),
		}

		rs.mirrorSessionToTimetable(context.Background(), activeGroup, []int64{101})

		assert.False(t, createCalled)
	})

	t.Run("lookup error", func(t *testing.T) {
		createCalled := false
		rs := &Resource{
			TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
				ActivityInstanceRepo: &mirrorInstanceRepoStub{
					findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
						return nil, errors.New("db down")
					},
					create: func(context.Context, *scheduleModel.ActivityInstance) error {
						createCalled = true
						return nil
					},
				},
				InstanceStaffRepo: &mirrorInstanceStaffRepoStub{},
			}),
		}

		rs.mirrorSessionToTimetable(context.Background(), activeGroup, []int64{101})

		assert.False(t, createCalled)
	})
}

func TestMirrorSessionToTimetableHandlesCreateFailures(t *testing.T) {
	groupID := int64(44)
	activeGroup := &active.Group{GroupID: &groupID, StartTime: time.Now()}
	activeGroup.ID = 66

	t.Run("instance create failure", func(t *testing.T) {
		staffRepo := &mirrorInstanceStaffRepoStub{}
		rs := &Resource{
			TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
				ActivityInstanceRepo: &mirrorInstanceRepoStub{
					findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
						return nil, nil
					},
					create: func(context.Context, *scheduleModel.ActivityInstance) error {
						return errors.New("insert failed")
					},
				},
				InstanceStaffRepo: staffRepo,
			}),
		}

		rs.mirrorSessionToTimetable(context.Background(), activeGroup, []int64{101})

		assert.Empty(t, staffRepo.created)
	})

	t.Run("staff create failure still attempts remaining staff", func(t *testing.T) {
		staffRepo := &mirrorInstanceStaffRepoStub{err: errors.New("staff insert failed")}
		rs := &Resource{
			TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
				ActivityInstanceRepo: &mirrorInstanceRepoStub{
					findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
						return nil, nil
					},
					create: func(_ context.Context, inst *scheduleModel.ActivityInstance) error {
						inst.ID = 77
						return nil
					},
				},
				InstanceStaffRepo: staffRepo,
			}),
		}

		rs.mirrorSessionToTimetable(context.Background(), activeGroup, []int64{101, 202})

		assert.Empty(t, staffRepo.created)
	})
}

func TestCompleteMirroredTimetableInstanceSkipsInvalidAndFailedLookups(t *testing.T) {
	// Nothing wired and nothing to complete are the two genuine no-ops.
	rs := &Resource{}
	require.NoError(t, rs.completeMirroredTimetableInstance(context.Background(), 0))
	require.NoError(t, rs.completeMirroredTimetableInstance(context.Background(), 66))

	completeCalled := false
	repo := &mirrorInstanceRepoStub{
		findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
			return nil, errors.New("db down")
		},
		completeActive: func(context.Context, []int64, time.Time) (int64, error) {
			completeCalled = true
			return 1, nil
		},
	}
	rs = &Resource{
		TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
			ActivityInstanceRepo: repo,
		}),
		TimetableBridge: mirrorBridge(repo),
	}
	// A failed lookup must surface: the caller rolls the session close back
	// with it rather than leaving the block running (#1747 review).
	require.Error(t, rs.completeMirroredTimetableInstance(context.Background(), 66))
	assert.False(t, completeCalled)

	repo = &mirrorInstanceRepoStub{
		findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
			return nil, nil
		},
		completeActive: func(context.Context, []int64, time.Time) (int64, error) {
			completeCalled = true
			return 1, nil
		},
	}
	rs.TimetableData = scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
		ActivityInstanceRepo: repo,
	})
	rs.TimetableBridge = mirrorBridge(repo)
	// No mirrored instance for this session: nothing to complete, no error.
	require.NoError(t, rs.completeMirroredTimetableInstance(context.Background(), 66))
	assert.False(t, completeCalled)
}

// Mirroring wired without its completion bridge is a wiring bug, and it fails
// loudly. Sessions started on this path DO create instances, so a silent skip
// would leak one permanently active block per kiosk session — and stamping the
// instance completed instead would leave its attendance unfinalized (#1747
// review).
func TestCompleteMirroredTimetableInstanceRequiresBridge(t *testing.T) {
	activeGroupID := int64(66)
	inst := &scheduleModel.ActivityInstance{ActiveGroupID: &activeGroupID}
	inst.ID = 77
	bc := testpkg.NewRecordingBroadcaster()
	rs := &Resource{
		TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
			ActivityInstanceRepo: &mirrorInstanceRepoStub{
				findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
					return inst, nil
				},
				markCompleted: func(context.Context, int64, time.Time) error {
					t.Fatal("the mirrored completion must never bypass the attendance bridge")
					return nil
				},
			},
		}),
		Broadcaster: bc,
	}

	err := rs.completeMirroredTimetableInstance(context.Background(), activeGroupID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "completion bridge")
	assert.Empty(t, bc.CallsByMethod("tenant"))
}

func TestCompleteMirroredTimetableInstanceStopsWhenCompletionFails(t *testing.T) {
	activeGroupID := int64(66)
	inst := &scheduleModel.ActivityInstance{ActiveGroupID: &activeGroupID}
	inst.ID = 77
	bc := testpkg.NewRecordingBroadcaster()
	repo := &mirrorInstanceRepoStub{
		findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
			return inst, nil
		},
		completeActive: func(context.Context, []int64, time.Time) (int64, error) {
			return 0, errors.New("update failed")
		},
	}
	rs := &Resource{
		TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
			ActivityInstanceRepo: repo,
		}),
		TimetableBridge: mirrorBridge(repo),
		Broadcaster:     bc,
	}

	// The bridge failing is what must reach the handler: it rolls the RFID
	// session close back instead of acknowledging a half-closed session.
	require.Error(t, rs.completeMirroredTimetableInstance(context.Background(), activeGroupID))

	assert.Empty(t, bc.CallsByMethod("tenant"))
}

// The kiosk's session end finalizes the attendance through the bridge and only
// then announces the completed instance (#1747 review).
func TestCompleteMirroredTimetableInstanceGoesThroughBridge(t *testing.T) {
	activeGroupID := int64(66)
	inst := &scheduleModel.ActivityInstance{
		Date:          timezone.NewDate(2026, 5, 11),
		StartTime:     time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		ActiveGroupID: &activeGroupID,
	}
	inst.ID = 77
	bc := testpkg.NewRecordingBroadcaster()
	var bridged []int64
	repo := &mirrorInstanceRepoStub{
		findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
			return inst, nil
		},
		completeActive: func(_ context.Context, ids []int64, _ time.Time) (int64, error) {
			bridged = ids
			return 1, nil
		},
	}
	rs := &Resource{
		TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
			ActivityInstanceRepo: repo,
		}),
		TimetableBridge: mirrorBridge(repo),
		Broadcaster:     bc,
	}

	ctx, drain := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 42))
	require.NoError(t, rs.completeMirroredTimetableInstance(ctx, activeGroupID))

	assert.Equal(t, []int64{activeGroupID}, bridged, "completion must go through the attendance bridge")
	assert.Equal(t, scheduleModel.InstanceStatusCompleted, inst.Status)
	require.NotNil(t, inst.CompletedAt)
	assert.Empty(t, bc.CallsByMethod("tenant"), "mirrored timetable SSE must wait for commit")

	drain()

	calls := bc.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, realtime.EventInstanceCompleted, calls[0].Event.Type)
}

func TestResourceRouterIsWired(t *testing.T) {
	rs := NewResource(nil, nil, nil, nil, nil, nil)
	router := rs.Router()

	require.NotNil(t, router)
}

func TestConfigureTimetableMirrorStoresDependencies(t *testing.T) {
	timetableData := scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{})
	bc := testpkg.NewRecordingBroadcaster()
	rs := &Resource{}

	bridge := scheduleSvc.NewTimetableBridgeService(scheduleSvc.TimetableBridgeDependencies{})
	rs.ConfigureTimetableMirror(timetableData, bridge, bc)

	assert.Same(t, timetableData, rs.TimetableData)
	assert.Same(t, bridge, rs.TimetableBridge)
	assert.Same(t, bc, rs.Broadcaster)
}
