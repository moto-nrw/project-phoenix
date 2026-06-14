package active

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type deviceRepoForSessionUnitTest struct {
	updateRoomIDFunc func(ctx context.Context, id int64, roomID int64) error
}

func (d *deviceRepoForSessionUnitTest) Create(context.Context, *iotModels.Device) error {
	return nil
}
func (d *deviceRepoForSessionUnitTest) FindByID(context.Context, interface{}) (*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) Update(context.Context, *iotModels.Device) error {
	return nil
}
func (d *deviceRepoForSessionUnitTest) Delete(context.Context, interface{}) error {
	return nil
}
func (d *deviceRepoForSessionUnitTest) List(context.Context, map[string]interface{}) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByDeviceID(context.Context, string) (*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByAPIKey(context.Context, string) (*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByType(context.Context, string) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByStatus(context.Context, iotModels.DeviceStatus) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindByRegisteredBy(context.Context, int64) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) UpdateLastSeen(context.Context, int64, time.Time) error {
	return nil
}
func (d *deviceRepoForSessionUnitTest) UpdateRoomID(ctx context.Context, id int64, roomID int64) error {
	if d.updateRoomIDFunc != nil {
		return d.updateRoomIDFunc(ctx, id, roomID)
	}
	return nil
}
func (d *deviceRepoForSessionUnitTest) UpdateStatus(context.Context, string, iotModels.DeviceStatus) error {
	return nil
}
func (d *deviceRepoForSessionUnitTest) FindActiveDevices(context.Context) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindDevicesRequiringMaintenance(context.Context) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) FindOfflineDevices(context.Context, time.Duration) ([]*iotModels.Device, error) {
	return nil, nil
}
func (d *deviceRepoForSessionUnitTest) CountDevicesByType(context.Context) (map[string]int, error) {
	return nil, nil
}

type settingsResolverForSessionUnitTest struct {
	has    bool
	hasErr error
	intVal int
	intErr error
}

func (s *settingsResolverForSessionUnitTest) HasTenantOverride(context.Context, string) (bool, error) {
	return s.has, s.hasErr
}
func (s *settingsResolverForSessionUnitTest) ResolveString(context.Context, string) (string, error) {
	return "", nil
}
func (s *settingsResolverForSessionUnitTest) ResolveInt(context.Context, string) (int, error) {
	return s.intVal, s.intErr
}

func TestProcessSessionTimeoutByID_ContinuesWhenSSECollectionFails(t *testing.T) {
	ctx := context.Background()
	activityID := int64(200)
	findCalls := 0
	endedVisits := 0

	svc := &service{
		logger: slog.Default(),
		groupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return &activeModels.Group{Model: modelBase.Model{ID: 100}, GroupID: &activityID}, nil
			},
		},
		visitRepo: &mockVisitRepository{
			findByActiveGroupIDFunc: func(context.Context, int64) ([]*activeModels.Visit, error) {
				findCalls++
				if findCalls == 1 {
					return nil, errors.New("student lookup prefetch failed")
				}
				return []*activeModels.Visit{
					{Model: modelBase.Model{ID: 300}, ActiveGroupID: 100, StudentID: 400},
				}, nil
			},
			endVisitFunc: func(context.Context, int64) error {
				endedVisits++
				return nil
			},
		},
	}

	result, err := svc.ProcessSessionTimeoutByID(ctx, 100)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, activityID, *result.ActivityID)
	assert.Equal(t, 1, result.StudentsCheckedOut)
	assert.Equal(t, 1, endedVisits)
}

func TestProcessSessionTimeoutByID_ReturnsCheckoutAndEndErrors(t *testing.T) {
	ctx := context.Background()
	activeGroup := &activeModels.Group{Model: modelBase.Model{ID: 100}}

	t.Run("checkout lookup failure", func(t *testing.T) {
		findCalls := 0
		svc := &service{
			groupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return activeGroup, nil
				},
			},
			visitRepo: &mockVisitRepository{
				findByActiveGroupIDFunc: func(context.Context, int64) ([]*activeModels.Visit, error) {
					findCalls++
					if findCalls == 1 {
						return []*activeModels.Visit{}, nil
					}
					return nil, errors.New("visit checkout lookup failed")
				},
			},
		}

		result, err := svc.ProcessSessionTimeoutByID(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "visit checkout lookup failed")
	})

	t.Run("session end failure", func(t *testing.T) {
		svc := &service{
			groupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return activeGroup, nil
				},
				endSessionFunc: func(context.Context, int64) error {
					return errors.New("session end failed")
				},
			},
			visitRepo: &mockVisitRepository{},
		}

		result, err := svc.ProcessSessionTimeoutByID(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "session end failed")
	})
}

func TestUpdateSessionActivity_RepositoryMissesAreMapped(t *testing.T) {
	ctx := context.Background()
	missErr := &modelBase.DatabaseError{Op: "update last activity - session not found", Err: sql.ErrNoRows}

	tests := []struct {
		name      string
		findFunc  func(context.Context, interface{}) (*activeModels.Group, error)
		wantError string
	}{
		{
			name: "find by id no rows becomes not found",
			findFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return nil, &modelBase.DatabaseError{Op: "find by id", Err: sql.ErrNoRows}
			},
			wantError: ErrActiveGroupNotFound.Error(),
		},
		{
			name: "nil session becomes not found",
			findFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return nil, nil
			},
			wantError: ErrActiveGroupNotFound.Error(),
		},
		{
			name: "ended session becomes already ended",
			findFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				endedAt := time.Now()
				return &activeModels.Group{Model: modelBase.Model{ID: 100}, EndTime: &endedAt}, nil
			},
			wantError: ErrActiveGroupAlreadyEnded.Error(),
		},
		{
			name: "unexpected lookup error is preserved",
			findFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return nil, errors.New("lookup exploded")
			},
			wantError: "lookup exploded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &service{
				groupRepo: &mockGroupRepository{
					updateLastActivityFunc: func(context.Context, int64, time.Time) error {
						return missErr
					},
					findByIDFunc: tt.findFunc,
				},
			}

			err := svc.UpdateSessionActivity(ctx, 100)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func TestUpdateSessionActivity_NonMissUpdateErrorIsPreserved(t *testing.T) {
	svc := &service{
		groupRepo: &mockGroupRepository{
			updateLastActivityFunc: func(context.Context, int64, time.Time) error {
				return errors.New("deadlock")
			},
		},
	}

	err := svc.UpdateSessionActivity(context.Background(), 100)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deadlock")
}

func TestSessionDeviceOnlineWindowResolution(t *testing.T) {
	ctx := context.Background()

	t.Run("default without settings", func(t *testing.T) {
		svc := &service{}
		assert.Equal(t, defaultDeviceOnlineWindow, svc.deviceOnlineWindow(ctx))
	})

	t.Run("default when override check fails", func(t *testing.T) {
		svc := &service{
			logger:   slog.Default(),
			settings: &settingsResolverForSessionUnitTest{hasErr: errors.New("settings db down")},
		}
		assert.Equal(t, defaultDeviceOnlineWindow, svc.deviceOnlineWindow(ctx))
	})

	t.Run("default without tenant override", func(t *testing.T) {
		svc := &service{settings: &settingsResolverForSessionUnitTest{has: false, intVal: 2}}
		assert.Equal(t, defaultDeviceOnlineWindow, svc.deviceOnlineWindow(ctx))
	})

	t.Run("default for invalid override", func(t *testing.T) {
		svc := &service{settings: &settingsResolverForSessionUnitTest{has: true, intVal: 0}}
		assert.Equal(t, defaultDeviceOnlineWindow, svc.deviceOnlineWindow(ctx))
	})

	t.Run("default when resolve fails", func(t *testing.T) {
		svc := &service{settings: &settingsResolverForSessionUnitTest{has: true, intErr: errors.New("missing")}}
		assert.Equal(t, defaultDeviceOnlineWindow, svc.deviceOnlineWindow(ctx))
	})

	t.Run("valid tenant override", func(t *testing.T) {
		svc := &service{settings: &settingsResolverForSessionUnitTest{has: true, intVal: 2}}
		assert.Equal(t, 2*time.Minute, svc.deviceOnlineWindow(ctx))
	})
}

func TestSessionIsDeviceOnline(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Minute)
	old := now.Add(-10 * time.Minute)
	svc := &service{settings: &settingsResolverForSessionUnitTest{has: true, intVal: 5}}

	assert.False(t, svc.isDeviceOnline(context.Background(), nil, now))
	assert.False(t, svc.isDeviceOnline(context.Background(), &iotModels.Device{}, now))
	assert.True(t, svc.isDeviceOnline(context.Background(), &iotModels.Device{LastSeen: &recent}, now))
	assert.False(t, svc.isDeviceOnline(context.Background(), &iotModels.Device{LastSeen: &old}, now))
}

func TestUpdateDeviceLocationBestEffort(t *testing.T) {
	calls := 0
	svc := &service{
		logger: slog.Default(),
		deviceRepo: &deviceRepoForSessionUnitTest{
			updateRoomIDFunc: func(context.Context, int64, int64) error {
				calls++
				return errors.New("device table temporarily unavailable")
			},
		},
	}

	svc.updateDeviceLocation(context.Background(), 50, 60)

	assert.Equal(t, 1, calls)
}

func TestCreateSessionBase_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("create failure is returned before side effects", func(t *testing.T) {
		expectedErr := errors.New("group insert failed")
		transferCalls := 0
		svc := &service{
			groupRepo: &mockGroupRepository{
				createFunc: func(context.Context, *activeModels.Group) error {
					return expectedErr
				},
			},
			visitRepo: &mockVisitRepository{
				transferVisitsFromRecentSessionsFunc: func(context.Context, int64, int64) (int, error) {
					transferCalls++
					return 0, nil
				},
			},
		}

		group, transferred, err := svc.createSessionBase(ctx, 10, 20, 30)

		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, group)
		assert.Zero(t, transferred)
		assert.Zero(t, transferCalls)
	})

	t.Run("transfer failure is returned after group creation", func(t *testing.T) {
		expectedErr := errors.New("visit transfer failed")
		svc := &service{
			groupRepo: &mockGroupRepository{
				createFunc: func(_ context.Context, group *activeModels.Group) error {
					group.ID = 44
					return nil
				},
			},
			visitRepo: &mockVisitRepository{
				transferVisitsFromRecentSessionsFunc: func(_ context.Context, newActiveGroupID, deviceID int64) (int, error) {
					assert.Equal(t, int64(44), newActiveGroupID)
					assert.Equal(t, int64(20), deviceID)
					return 0, expectedErr
				},
			},
			deviceRepo: &deviceRepoForSessionUnitTest{},
		}

		group, transferred, err := svc.createSessionBase(ctx, 10, 20, 30)

		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, group)
		assert.Zero(t, transferred)
	})

	t.Run("success without device skips location update", func(t *testing.T) {
		locationUpdates := 0
		svc := &service{
			groupRepo: &mockGroupRepository{
				createFunc: func(_ context.Context, group *activeModels.Group) error {
					group.ID = 45
					return nil
				},
			},
			visitRepo: &mockVisitRepository{
				transferVisitsFromRecentSessionsFunc: func(_ context.Context, newActiveGroupID, deviceID int64) (int, error) {
					assert.Equal(t, int64(45), newActiveGroupID)
					assert.Zero(t, deviceID)
					return 2, nil
				},
			},
			deviceRepo: &deviceRepoForSessionUnitTest{
				updateRoomIDFunc: func(context.Context, int64, int64) error {
					locationUpdates++
					return nil
				},
			},
		}

		group, transferred, err := svc.createSessionBase(ctx, 10, 0, 30)

		require.NoError(t, err)
		require.NotNil(t, group)
		assert.Equal(t, int64(45), group.ID)
		assert.Equal(t, 2, transferred)
		assert.Zero(t, locationUpdates)
	})
}

func TestCreateSessionWithSupervisor_TransferredVisitsBranch(t *testing.T) {
	svc := &service{
		logger: slog.Default(),
		groupRepo: &mockGroupRepository{
			createFunc: func(_ context.Context, group *activeModels.Group) error {
				group.ID = 50
				return nil
			},
		},
		visitRepo: &mockVisitRepository{
			transferVisitsFromRecentSessionsFunc: func(context.Context, int64, int64) (int, error) {
				return 3, nil
			},
		},
		supervisorRepo: &mockGroupSupervisorRepository{},
		deviceRepo:     &deviceRepoForSessionUnitTest{},
	}

	group, err := svc.createSessionWithSupervisor(context.Background(), 11, 22, 33, 44)

	require.NoError(t, err)
	require.NotNil(t, group)
	assert.Equal(t, int64(50), group.ID)
}

func TestCreateSessionWithMultipleSupervisors_TransferredVisitsBranch(t *testing.T) {
	var assigned []int64
	svc := &service{
		logger: slog.Default(),
		groupRepo: &mockGroupRepository{
			createFunc: func(_ context.Context, group *activeModels.Group) error {
				group.ID = 60
				return nil
			},
		},
		visitRepo: &mockVisitRepository{
			transferVisitsFromRecentSessionsFunc: func(context.Context, int64, int64) (int, error) {
				return 2, nil
			},
		},
		supervisorRepo: &mockGroupSupervisorRepository{
			createFunc: func(_ context.Context, supervisor *activeModels.GroupSupervisor) error {
				assigned = append(assigned, supervisor.StaffID)
				return nil
			},
		},
		deviceRepo: &deviceRepoForSessionUnitTest{},
	}

	group, err := svc.createSessionWithMultipleSupervisors(context.Background(), 11, 22, []int64{33, 44, 33}, 55)

	require.NoError(t, err)
	require.NotNil(t, group)
	assert.ElementsMatch(t, []int64{33, 44}, assigned)
}

func TestManualRoomSelectionStrategies(t *testing.T) {
	ctx := context.Background()

	t.Run("ignore strategy skips conflict check", func(t *testing.T) {
		svc := &service{}
		roomID, err := svc.validateManualRoomSelection(ctx, 10, RoomConflictIgnore)
		require.NoError(t, err)
		assert.Equal(t, int64(10), roomID)
	})

	t.Run("repository error is returned", func(t *testing.T) {
		svc := &service{
			groupRepo: &mockGroupRepository{
				checkRoomConflictFunc: func(context.Context, int64, int64) (bool, *activeModels.Group, error) {
					return false, nil, errors.New("conflict lookup failed")
				},
			},
		}

		roomID, err := svc.validateManualRoomSelection(ctx, 10, RoomConflictFail)

		require.Error(t, err)
		assert.Zero(t, roomID)
		assert.Contains(t, err.Error(), "conflict lookup failed")
	})

	t.Run("warn strategy permits conflict", func(t *testing.T) {
		svc := &service{
			logger: slog.Default(),
			groupRepo: &mockGroupRepository{
				checkRoomConflictFunc: func(context.Context, int64, int64) (bool, *activeModels.Group, error) {
					return true, &activeModels.Group{Model: modelBase.Model{ID: 20}}, nil
				},
			},
		}

		roomID, err := svc.validateManualRoomSelection(ctx, 10, RoomConflictWarn)

		require.NoError(t, err)
		assert.Equal(t, int64(10), roomID)
	})
}

func TestEndExistingActivitySessionsForForceStart_SkipsInvalidRowsAndStopsOnErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("skips nil and invalid sessions", func(t *testing.T) {
		var ended []int64
		svc := &service{
			groupRepo: &mockGroupRepository{
				findActiveByGroupIDFunc: func(context.Context, int64) ([]*activeModels.Group, error) {
					return []*activeModels.Group{
						nil,
						{Model: modelBase.Model{ID: 0}},
						{Model: modelBase.Model{ID: 30}},
					}, nil
				},
				endSessionFunc: func(_ context.Context, id int64) error {
					ended = append(ended, id)
					return nil
				},
			},
		}

		endedIDs, err := svc.endExistingActivitySessionsForForceStart(ctx, 500)

		require.NoError(t, err)
		assert.Equal(t, []int64{30}, ended)
		assert.Equal(t, []int64{30}, endedIDs)
	})

	t.Run("find error is returned", func(t *testing.T) {
		svc := &service{
			groupRepo: &mockGroupRepository{
				findActiveByGroupIDFunc: func(context.Context, int64) ([]*activeModels.Group, error) {
					return nil, errors.New("active sessions lookup failed")
				},
			},
		}

		endedIDs, err := svc.endExistingActivitySessionsForForceStart(ctx, 500)

		require.Error(t, err)
		assert.Nil(t, endedIDs)
		assert.Contains(t, err.Error(), "active sessions lookup failed")
	})

	t.Run("end error stops immediately", func(t *testing.T) {
		expectedErr := errors.New("end active session failed")
		svc := &service{
			groupRepo: &mockGroupRepository{
				findActiveByGroupIDFunc: func(context.Context, int64) ([]*activeModels.Group, error) {
					return []*activeModels.Group{{Model: modelBase.Model{ID: 31}}}, nil
				},
				endSessionFunc: func(context.Context, int64) error {
					return expectedErr
				},
			},
		}

		endedIDs, err := svc.endExistingActivitySessionsForForceStart(ctx, 500)

		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, endedIDs)
	})
}

func TestTransferForceStartedActivityState_PropagatesTransferErrors(t *testing.T) {
	ctx := context.Background()
	visitErr := errors.New("visit update failed")
	supervisorErr := errors.New("supervisor lookup failed")

	t.Run("visit transfer error", func(t *testing.T) {
		svc := &service{
			visitRepo: &mockVisitRepository{
				transferActiveVisitsBetweenGroupsFunc: func(context.Context, int64, int64) (int, error) {
					return 0, visitErr
				},
			},
		}

		err := svc.transferForceStartedActivityState(ctx, []int64{1}, 2, time.Now())

		require.ErrorIs(t, err, visitErr)
	})

	t.Run("supervisor transfer error", func(t *testing.T) {
		svc := &service{
			visitRepo: &mockVisitRepository{},
			supervisorRepo: &mockGroupSupervisorRepository{
				findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
					return nil, supervisorErr
				},
			},
		}

		err := svc.transferForceStartedActivityState(ctx, []int64{1}, 2, time.Now())

		require.ErrorIs(t, err, supervisorErr)
	})
}

func TestTransferActiveSupervisorsBetweenGroups_ErrorBranches(t *testing.T) {
	ctx := context.Background()
	today := timezone.TodayDate()

	t.Run("new supervisor lookup error", func(t *testing.T) {
		call := 0
		expectedErr := errors.New("new supervisor lookup failed")
		svc := &service{
			supervisorRepo: &mockGroupSupervisorRepository{
				findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
					call++
					if call == 2 {
						return nil, expectedErr
					}
					return []*activeModels.GroupSupervisor{}, nil
				},
			},
		}

		count, err := svc.transferActiveSupervisorsBetweenGroups(ctx, 1, 2, time.Now())

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, count)
	})

	t.Run("end supervision error returns partial count", func(t *testing.T) {
		expectedErr := errors.New("end supervision failed")
		svc := &service{
			supervisorRepo: &mockGroupSupervisorRepository{
				findByActiveGroupIDFunc: func(_ context.Context, activeGroupID int64, _ bool) ([]*activeModels.GroupSupervisor, error) {
					if activeGroupID == 1 {
						return []*activeModels.GroupSupervisor{
							{Model: modelBase.Model{ID: 10}, StaffID: 20, Role: "helper", StartDate: today},
						}, nil
					}
					return []*activeModels.GroupSupervisor{}, nil
				},
				endSupervisionFunc: func(context.Context, int64) error {
					return expectedErr
				},
			},
		}

		count, err := svc.transferActiveSupervisorsBetweenGroups(ctx, 1, 2, time.Now())

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, count)
	})
}

func TestTransferActiveVisitsBetweenGroups_DelegatesToConditionalRepositoryTransfer(t *testing.T) {
	ctx := context.Background()
	var gotOldGroupID, gotNewGroupID int64
	svc := &service{
		visitRepo: &mockVisitRepository{
			transferActiveVisitsBetweenGroupsFunc: func(_ context.Context, oldGroupID, newGroupID int64) (int, error) {
				gotOldGroupID = oldGroupID
				gotNewGroupID = newGroupID
				return 3, nil
			},
		},
	}

	count, err := svc.transferActiveVisitsBetweenGroups(ctx, 100, 200)

	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Equal(t, int64(100), gotOldGroupID)
	assert.Equal(t, int64(200), gotNewGroupID)
}

func TestEndExistingDeviceSession_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("find error is returned", func(t *testing.T) {
		expectedErr := errors.New("device session lookup failed")
		svc := &service{
			groupRepo: &mockGroupRepository{
				findActiveByDeviceIDFunc: func(context.Context, int64) (*activeModels.Group, error) {
					return nil, expectedErr
				},
			},
		}

		err := svc.endExistingDeviceSession(ctx, 100, false)

		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("nil existing session is a no-op", func(t *testing.T) {
		svc := &service{groupRepo: &mockGroupRepository{}}

		err := svc.endExistingDeviceSession(ctx, 100, false)

		require.NoError(t, err)
	})

	t.Run("full cleanup delegates to EndActivitySession", func(t *testing.T) {
		endedGroupIDs := []int64{}
		svc := &service{
			groupRepo: &mockGroupRepository{
				findActiveByDeviceIDFunc: func(context.Context, int64) (*activeModels.Group, error) {
					return &activeModels.Group{Model: modelBase.Model{ID: 300}}, nil
				},
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return &activeModels.Group{Model: modelBase.Model{ID: 300}}, nil
				},
				endSessionFunc: func(_ context.Context, id int64) error {
					endedGroupIDs = append(endedGroupIDs, id)
					return nil
				},
			},
			visitRepo:      &mockVisitRepository{},
			supervisorRepo: &mockGroupSupervisorRepository{},
		}

		err := svc.endExistingDeviceSession(ctx, 100, true)

		require.NoError(t, err)
		assert.Equal(t, []int64{300}, endedGroupIDs)
	})

	t.Run("simple cleanup ends the device session directly", func(t *testing.T) {
		var ended []int64
		svc := &service{
			groupRepo: &mockGroupRepository{
				findActiveByDeviceIDFunc: func(context.Context, int64) (*activeModels.Group, error) {
					return &activeModels.Group{Model: modelBase.Model{ID: 301}}, nil
				},
				endSessionFunc: func(_ context.Context, id int64) error {
					ended = append(ended, id)
					return nil
				},
			},
		}

		err := svc.endExistingDeviceSession(ctx, 100, false)

		require.NoError(t, err)
		assert.Equal(t, []int64{301}, ended)
	})
}

func TestEndExistingDeviceSessionForForceStart_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("find error is returned", func(t *testing.T) {
		expectedErr := errors.New("force device lookup failed")
		svc := &service{
			groupRepo: &mockGroupRepository{
				findActiveByDeviceIDFunc: func(context.Context, int64) (*activeModels.Group, error) {
					return nil, expectedErr
				},
			},
		}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, endedID)
	})

	t.Run("nil existing session is zero", func(t *testing.T) {
		svc := &service{groupRepo: &mockGroupRepository{}}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.NoError(t, err)
		assert.Zero(t, endedID)
	})

	t.Run("end error is returned", func(t *testing.T) {
		expectedErr := errors.New("force end failed")
		svc := &service{
			groupRepo: &mockGroupRepository{
				findActiveByDeviceIDFunc: func(context.Context, int64) (*activeModels.Group, error) {
					return &activeModels.Group{Model: modelBase.Model{ID: 302}}, nil
				},
				endSessionFunc: func(context.Context, int64) error {
					return expectedErr
				},
			},
		}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.ErrorIs(t, err, expectedErr)
		assert.Zero(t, endedID)
	})

	t.Run("returns ended session id", func(t *testing.T) {
		svc := &service{
			groupRepo: &mockGroupRepository{
				findActiveByDeviceIDFunc: func(context.Context, int64) (*activeModels.Group, error) {
					return &activeModels.Group{Model: modelBase.Model{ID: 303}}, nil
				},
			},
		}

		endedID, err := svc.endExistingDeviceSessionForForceStart(ctx, 100)

		require.NoError(t, err)
		assert.Equal(t, int64(303), endedID)
	})
}

func TestAppendActiveGroupID_DeduplicatesAndSkipsInvalid(t *testing.T) {
	ids := appendActiveGroupID(nil, 0)
	ids = appendActiveGroupID(ids, -1)
	ids = appendActiveGroupID(ids, 10)
	ids = appendActiveGroupID(ids, 10)
	ids = appendActiveGroupIDs(ids, 11, 10, 12)

	assert.Equal(t, []int64{10, 11, 12}, ids)
}

func TestSupervisorReplacement_ErrorBranches(t *testing.T) {
	ctx := context.Background()
	startDate := timezone.TodayDate()

	t.Run("current supervisor lookup error", func(t *testing.T) {
		expectedErr := errors.New("current supervisors failed")
		svc := &service{
			supervisorRepo: &mockGroupSupervisorRepository{
				findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
					return nil, expectedErr
				},
			},
		}

		err := svc.replaceSupervisorsInTransaction(ctx, 100, map[int64]bool{10: true})

		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("ending current supervisor error", func(t *testing.T) {
		expectedErr := errors.New("end current supervisor failed")
		svc := &service{
			supervisorRepo: &mockGroupSupervisorRepository{
				findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
					return []*activeModels.GroupSupervisor{
						{Model: modelBase.Model{ID: 20}, StaffID: 10, Role: "supervisor", StartDate: startDate},
					}, nil
				},
				updateFunc: func(context.Context, *activeModels.GroupSupervisor) error {
					return expectedErr
				},
			},
		}

		err := svc.replaceSupervisorsInTransaction(ctx, 100, map[int64]bool{10: true})

		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("reactivate existing supervisor update error", func(t *testing.T) {
		expectedErr := errors.New("reactivate failed")
		endedDate := startDate.AddDays(-1)
		supervisors := []*activeModels.GroupSupervisor{
			{Model: modelBase.Model{ID: 20}, StaffID: 10, Role: "supervisor", StartDate: startDate, EndDate: &endedDate},
		}
		updateCalls := 0
		svc := &service{
			supervisorRepo: &mockGroupSupervisorRepository{
				findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
					return supervisors, nil
				},
				updateFunc: func(context.Context, *activeModels.GroupSupervisor) error {
					updateCalls++
					if updateCalls == 2 {
						return expectedErr
					}
					return nil
				},
			},
		}

		err := svc.replaceSupervisorsInTransaction(ctx, 100, map[int64]bool{10: true})

		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("create new supervisor error", func(t *testing.T) {
		expectedErr := errors.New("create supervisor failed")
		svc := &service{
			supervisorRepo: &mockGroupSupervisorRepository{
				findByActiveGroupIDFunc: func(context.Context, int64, bool) ([]*activeModels.GroupSupervisor, error) {
					return []*activeModels.GroupSupervisor{}, nil
				},
				createFunc: func(context.Context, *activeModels.GroupSupervisor) error {
					return expectedErr
				},
			},
		}

		err := svc.replaceSupervisorsInTransaction(ctx, 100, map[int64]bool{10: true})

		require.ErrorIs(t, err, expectedErr)
	})
}

func TestGetDeviceIDString(t *testing.T) {
	deviceID := int64(42)

	assert.Equal(t, "unknown", getDeviceIDString(nil))
	assert.Equal(t, "42", getDeviceIDString(&deviceID))
}

func TestNormalizeTransferredSupervisorRole(t *testing.T) {
	assert.Equal(t, "supervisor", normalizeTransferredSupervisorRole("Supervisor"))
	assert.Equal(t, "helper", normalizeTransferredSupervisorRole("helper"))
}

func TestValidateSessionForTimeout_Branches(t *testing.T) {
	ctx := context.Background()

	t.Run("not found", func(t *testing.T) {
		svc := &service{
			groupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return nil, errors.New("missing")
				},
			},
		}

		session, err := svc.validateSessionForTimeout(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, session)
		assert.Contains(t, err.Error(), ErrActiveGroupNotFound.Error())
	})

	t.Run("already ended", func(t *testing.T) {
		endedAt := time.Now()
		svc := &service{
			groupRepo: &mockGroupRepository{
				findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
					return &activeModels.Group{Model: modelBase.Model{ID: 100}, EndTime: &endedAt}, nil
				},
			},
		}

		session, err := svc.validateSessionForTimeout(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, session)
		assert.Contains(t, err.Error(), ErrActiveGroupAlreadyEnded.Error())
	})
}

func TestEndDailySessions_RepositoryFailures(t *testing.T) {
	ctx := context.Background()
	activeGroup := &activeModels.Group{Model: modelBase.Model{ID: 100}}

	t.Run("list failure returns active error", func(t *testing.T) {
		svc := &service{
			groupRepo: &mockGroupRepository{
				listFunc: func(context.Context, *modelBase.QueryOptions) ([]*activeModels.Group, error) {
					return nil, errors.New("list failed")
				},
			},
		}

		result, err := svc.EndDailySessions(ctx)

		require.Error(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Success)
	})

	t.Run("visit bulk failure aborts later bulk steps", func(t *testing.T) {
		svc := &service{
			groupRepo: &mockGroupRepository{
				listFunc: func(context.Context, *modelBase.QueryOptions) ([]*activeModels.Group, error) {
					return []*activeModels.Group{activeGroup}, nil
				},
			},
			visitRepo: &mockVisitRepository{
				endVisitsByActiveGroupIDsFunc: func(context.Context, []int64) (int64, error) {
					return 0, errors.New("bulk visit close failed")
				},
			},
			supervisorRepo: &mockGroupSupervisorRepository{},
		}

		result, err := svc.EndDailySessions(ctx)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.Success)
		assert.Contains(t, result.Errors[0], "bulk visit close failed")
	})

	t.Run("session bulk failure records error and still tries supervisors", func(t *testing.T) {
		svc := &service{
			groupRepo: &mockGroupRepository{
				listFunc: func(context.Context, *modelBase.QueryOptions) ([]*activeModels.Group, error) {
					return []*activeModels.Group{activeGroup}, nil
				},
				endSessionsByIDsFunc: func(context.Context, []int64) (int64, error) {
					return 0, errors.New("bulk session close failed")
				},
			},
			visitRepo: &mockVisitRepository{
				endVisitsByActiveGroupIDsFunc: func(context.Context, []int64) (int64, error) {
					return 2, nil
				},
			},
			supervisorRepo: &mockGroupSupervisorRepository{
				endSupervisionsByIDsFunc: func(context.Context, []int64) (int64, error) {
					return 3, nil
				},
			},
		}

		result, err := svc.EndDailySessions(ctx)

		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Equal(t, 2, result.VisitsEnded)
		assert.Equal(t, 3, result.SupervisorsEnded)
		assert.Contains(t, result.Errors[0], "bulk session close failed")
	})

	t.Run("supervisor bulk failure records error", func(t *testing.T) {
		svc := &service{
			groupRepo: &mockGroupRepository{
				listFunc: func(context.Context, *modelBase.QueryOptions) ([]*activeModels.Group, error) {
					return []*activeModels.Group{activeGroup}, nil
				},
				endSessionsByIDsFunc: func(context.Context, []int64) (int64, error) {
					return 1, nil
				},
			},
			visitRepo: &mockVisitRepository{
				endVisitsByActiveGroupIDsFunc: func(context.Context, []int64) (int64, error) {
					return 2, nil
				},
			},
			supervisorRepo: &mockGroupSupervisorRepository{
				endSupervisionsByIDsFunc: func(context.Context, []int64) (int64, error) {
					return 0, errors.New("bulk supervisor close failed")
				},
			},
		}

		result, err := svc.EndDailySessions(ctx)

		require.NoError(t, err)
		assert.False(t, result.Success)
		assert.Equal(t, 1, result.SessionsEnded)
		assert.Contains(t, result.Errors[0], "bulk supervisor close failed")
	})
}

func TestCleanupOrphanedSupervisors_ErrorBranches(t *testing.T) {
	ctx := context.Background()
	today := timezone.TodayDate()

	t.Run("find failure is captured", func(t *testing.T) {
		result := &DailySessionCleanupResult{Success: true}
		svc := &service{
			supervisorRepo: &mockGroupSupervisorRepository{
				findStaleOpenFunc: func(context.Context, timezone.Date) ([]*activeModels.GroupSupervisor, error) {
					return nil, errors.New("stale lookup failed")
				},
			},
		}

		svc.cleanupOrphanedSupervisors(ctx, result)

		assert.False(t, result.Success)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0], "stale lookup failed")
	})

	t.Run("update failure is captured", func(t *testing.T) {
		result := &DailySessionCleanupResult{Success: true}
		svc := &service{
			supervisorRepo: &mockGroupSupervisorRepository{
				findStaleOpenFunc: func(context.Context, timezone.Date) ([]*activeModels.GroupSupervisor, error) {
					return []*activeModels.GroupSupervisor{
						{Model: modelBase.Model{ID: 10}, StartDate: today.AddDays(-1)},
					}, nil
				},
				updateColumnsFunc: func(context.Context, *activeModels.GroupSupervisor, ...string) (int64, error) {
					return 0, errors.New("stale close failed")
				},
			},
		}

		svc.cleanupOrphanedSupervisors(ctx, result)

		assert.False(t, result.Success)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0], "stale close failed")
	})
}
