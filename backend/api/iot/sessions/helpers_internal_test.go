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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureMirrorBroadcaster struct {
	called   bool
	tenantID int64
	event    realtime.Event
}

func (b *captureMirrorBroadcaster) BroadcastToGroup(_ int64, _ string, _ realtime.Event) error {
	return nil
}

func (b *captureMirrorBroadcaster) BroadcastParentMessage(_, _ int64, _ realtime.Event) error {
	return nil
}

func (b *captureMirrorBroadcaster) BroadcastToTenant(tenantID int64, event realtime.Event) error {
	b.called = true
	b.tenantID = tenantID
	b.event = event
	return nil
}

func (b *captureMirrorBroadcaster) BroadcastToAll(_ realtime.Event) error { return nil }

type mirrorInstanceRepoStub struct {
	scheduleModel.ActivityInstanceRepository
	findByActiveGroupID func(context.Context, int64) (*scheduleModel.ActivityInstance, error)
	create              func(context.Context, *scheduleModel.ActivityInstance) error
	markCompleted       func(context.Context, int64, time.Time) error
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
	bc := &captureMirrorBroadcaster{}
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

	assert.False(t, bc.called, "mirrored timetable SSE must wait for commit")

	drain()

	assert.True(t, bc.called)
	assert.Equal(t, int64(42), bc.tenantID)
	assert.Equal(t, realtime.EventInstanceStarted, bc.event.Type)
	assert.Equal(t, "321", bc.event.ActiveGroupID)
	requireData := bc.event.Data
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
	bc := &captureMirrorBroadcaster{}
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
	assert.False(t, bc.called)

	drain()

	assert.True(t, bc.called)
	assert.Equal(t, realtime.EventInstanceStarted, bc.event.Type)
}

func TestCompleteMirroredTimetableInstanceMarksCompletedAndBroadcasts(t *testing.T) {
	ctx, drain := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 42))
	activeGroupID := int64(66)
	inst := &scheduleModel.ActivityInstance{
		RoomID:        55,
		Date:          timezone.NewDate(2026, 5, 12),
		StartTime:     time.Date(2000, 1, 1, 14, 15, 0, 0, time.UTC),
		ActiveGroupID: &activeGroupID,
	}
	inst.ID = 77
	var completedID int64
	instanceRepo := &mirrorInstanceRepoStub{
		findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
			return inst, nil
		},
		markCompleted: func(_ context.Context, instanceID int64, _ time.Time) error {
			completedID = instanceID
			return nil
		},
	}
	bc := &captureMirrorBroadcaster{}
	rs := &Resource{TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{ActivityInstanceRepo: instanceRepo}), Broadcaster: bc}

	rs.completeMirroredTimetableInstance(ctx, activeGroupID)

	assert.Equal(t, int64(77), completedID)
	assert.Equal(t, scheduleModel.InstanceStatusCompleted, inst.Status)
	require.NotNil(t, inst.CompletedAt)
	assert.False(t, bc.called)

	drain()

	assert.True(t, bc.called)
	assert.Equal(t, realtime.EventInstanceCompleted, bc.event.Type)
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
	rs := &Resource{}
	rs.completeMirroredTimetableInstance(context.Background(), 0)
	rs.completeMirroredTimetableInstance(context.Background(), 66)

	markCalled := false
	rs = &Resource{
		TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
			ActivityInstanceRepo: &mirrorInstanceRepoStub{
				findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
					return nil, errors.New("db down")
				},
				markCompleted: func(context.Context, int64, time.Time) error {
					markCalled = true
					return nil
				},
			},
		}),
	}
	rs.completeMirroredTimetableInstance(context.Background(), 66)
	assert.False(t, markCalled)

	rs.TimetableData = scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
		ActivityInstanceRepo: &mirrorInstanceRepoStub{
			findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
				return nil, nil
			},
			markCompleted: func(context.Context, int64, time.Time) error {
				markCalled = true
				return nil
			},
		},
	})
	rs.completeMirroredTimetableInstance(context.Background(), 66)
	assert.False(t, markCalled)
}

func TestCompleteMirroredTimetableInstanceStopsWhenMarkCompletedFails(t *testing.T) {
	activeGroupID := int64(66)
	inst := &scheduleModel.ActivityInstance{ActiveGroupID: &activeGroupID}
	inst.ID = 77
	bc := &captureMirrorBroadcaster{}
	rs := &Resource{
		TimetableData: scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{
			ActivityInstanceRepo: &mirrorInstanceRepoStub{
				findByActiveGroupID: func(context.Context, int64) (*scheduleModel.ActivityInstance, error) {
					return inst, nil
				},
				markCompleted: func(context.Context, int64, time.Time) error {
					return errors.New("update failed")
				},
			},
		}),
		Broadcaster: bc,
	}

	rs.completeMirroredTimetableInstance(context.Background(), activeGroupID)

	assert.False(t, bc.called)
}

func TestResourceRouterIsWired(t *testing.T) {
	rs := NewResource(nil, nil, nil, nil, nil, nil)
	router := rs.Router()

	require.NotNil(t, router)
}

func TestConfigureTimetableMirrorStoresDependencies(t *testing.T) {
	timetableData := scheduleSvc.NewTimetableDataService(scheduleSvc.TimetableDataDependencies{})
	bc := &captureMirrorBroadcaster{}
	rs := &Resource{}

	rs.ConfigureTimetableMirror(timetableData, bc)

	assert.Same(t, timetableData, rs.TimetableData)
	assert.Same(t, bc, rs.Broadcaster)
}
