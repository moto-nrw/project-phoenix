package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lockingAttendanceRepository struct {
	StudentPresence
	calls []string
	row   studentpresence.Attendance
}

func (r *lockingAttendanceRepository) LockStudentAttendance(context.Context, int64) error {
	r.calls = append(r.calls, "lock")
	return nil
}

func (r *lockingAttendanceRepository) ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error) {
	r.calls = append(r.calls, "find")
	return []studentpresence.Attendance{r.row}, nil
}

func (r *lockingAttendanceRepository) ReviseAttendance(_ context.Context, row studentpresence.Attendance) (studentpresence.Attendance, error) {
	r.calls = append(r.calls, "update")
	return row, nil
}

type recordingAttendanceSyncer struct {
	checkInErr  error
	mirrorAtErr error
	loaded      []*studentpresence.Visit
	mirrored    []*studentpresence.Visit
	revised     [][2]*studentpresence.Visit
	mirrorAt    []struct {
		studentID int64
		at        time.Time
	}
}

func (r *recordingAttendanceSyncer) MirrorCheckInForVisit(_ context.Context, visit *studentpresence.Visit) (*AttendanceSnapshot, error) {
	copy := *visit
	r.mirrored = append(r.mirrored, &copy)
	if r.checkInErr != nil {
		return nil, r.checkInErr
	}
	return &AttendanceSnapshot{Status: "present", InstanceID: 2}, nil
}

func (r *recordingAttendanceSyncer) MirrorCheckInAt(_ context.Context, studentID int64, at time.Time) (*AttendanceSnapshot, error) {
	r.mirrorAt = append(r.mirrorAt, struct {
		studentID int64
		at        time.Time
	}{studentID: studentID, at: at})
	return nil, r.mirrorAtErr
}

func (r *recordingAttendanceSyncer) MirrorCheckOutForVisit(_ context.Context, visit *studentpresence.Visit) (*AttendanceSnapshot, error) {
	copy := *visit
	r.loaded = append(r.loaded, &copy)
	return &AttendanceSnapshot{Status: "present", InstanceID: 1}, nil
}

func (r *recordingAttendanceSyncer) MirrorVisitRevision(_ context.Context, previous, updated *studentpresence.Visit) error {
	previousCopy := *previous
	updatedCopy := *updated
	r.revised = append(r.revised, [2]*studentpresence.Visit{&previousCopy, &updatedCopy})
	return nil
}

func (r *recordingAttendanceSyncer) MirrorCheckOutAt(context.Context, int64, time.Time) error {
	return nil
}

func (r *recordingAttendanceSyncer) MirrorCheckInAtBatch(context.Context, []int64, time.Time) error {
	return nil
}

func (r *recordingAttendanceSyncer) MirrorCheckOutAtBatch(context.Context, []int64, time.Time) error {
	return nil
}

func (r *recordingAttendanceSyncer) MirrorCheckOutForVisits(_ context.Context, visits []*studentpresence.Visit, _ time.Time) error {
	for _, visit := range visits {
		copy := *visit
		r.loaded = append(r.loaded, &copy)
	}
	return nil
}

type failingTransferCheckoutSyncer struct {
	recordingAttendanceSyncer
	failAt int
	err    error
}

func (r *failingTransferCheckoutSyncer) MirrorCheckOutForVisit(ctx context.Context, visit *studentpresence.Visit) (*AttendanceSnapshot, error) {
	snapshot, err := r.recordingAttendanceSyncer.MirrorCheckOutForVisit(ctx, visit)
	if len(r.loaded) == r.failAt {
		return nil, r.err
	}
	return snapshot, err
}

func TestPresenceVisitValidation(t *testing.T) {
	t.Parallel()

	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)
	pastTime := nowTime.Add(-2 * time.Hour)

	tests := []struct {
		name    string
		visit   *studentpresence.Visit
		wantErr bool
	}{
		{
			name: "Valid visit",
			visit: &studentpresence.Visit{
				StudentID:     1,
				ActiveGroupID: 1,
				EntryTime:     nowTime,
			},
			wantErr: false,
		},
		{
			name: "Valid visit with exit time",
			visit: &studentpresence.Visit{
				StudentID:     1,
				ActiveGroupID: 1,
				EntryTime:     nowTime,
				ExitTime:      &futureTime,
			},
			wantErr: false,
		},
		{
			name: "Missing student ID",
			visit: &studentpresence.Visit{
				ActiveGroupID: 1,
				EntryTime:     nowTime,
			},
			wantErr: true,
		},
		{
			name: "Missing active group ID",
			visit: &studentpresence.Visit{
				StudentID: 1,
				EntryTime: nowTime,
			},
			wantErr: true,
		},
		{
			name: "Missing entry time",
			visit: &studentpresence.Visit{
				StudentID:     1,
				ActiveGroupID: 1,
			},
			wantErr: true,
		},
		{
			name: "Exit time before entry time",
			visit: &studentpresence.Visit{
				StudentID:     1,
				ActiveGroupID: 1,
				EntryTime:     nowTime,
				ExitTime:      &pastTime,
			},
			wantErr: true,
		},
		{
			name: "Invalid student ID",
			visit: &studentpresence.Visit{
				StudentID:     -1,
				ActiveGroupID: 1,
				EntryTime:     nowTime,
			},
			wantErr: true,
		},
		{
			name: "Invalid active group ID",
			visit: &studentpresence.Visit{
				StudentID:     1,
				ActiveGroupID: 0,
				EntryTime:     nowTime,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, !tt.wantErr, validPresenceVisit(tt.visit))
			if tt.wantErr {
				svc := &service{}
				require.ErrorIs(t, svc.createVisit(context.Background(), tt.visit), ErrInvalidData)
				require.ErrorIs(t, svc.updateVisit(context.Background(), tt.visit), ErrInvalidData)
			}
		})
	}
}

func TestVisitTransferPropagatesSourceAndTargetCheckoutSyncFailures(t *testing.T) {
	t.Parallel()
	for _, stage := range []struct {
		name   string
		failAt int
	}{{"source", 1}, {"target", 2}} {
		t.Run(stage.name, func(t *testing.T) {
			injected := errors.New("transfer checkout sync failed")
			syncer := &failingTransferCheckoutSyncer{failAt: stage.failAt, err: injected}
			svc := &service{ServiceDependencies: ServiceDependencies{AttendanceSyncer: syncer}}
			at := time.Now()
			previous := &studentpresence.Visit{StudentID: 1, ActiveGroupID: 2, EntryTime: at.Add(-time.Hour)}
			updated := &studentpresence.Visit{StudentID: 1, ActiveGroupID: 3, EntryTime: previous.EntryTime, ExitTime: &at}

			source, target, err := svc.syncMovedVisitAttendance(context.Background(), previous, updated, at)

			require.ErrorIs(t, err, injected)
			assert.Nil(t, source)
			assert.Nil(t, target)
			assert.Len(t, syncer.loaded, stage.failAt)
			assert.Len(t, syncer.mirrored, stage.failAt-1, "source failure must stop before target check-in")
		})
	}
}

func TestVisitTransferPropagatesTargetCheckInFailure(t *testing.T) {
	t.Parallel()
	injected := errors.New("target check-in sync failed")
	syncer := &recordingAttendanceSyncer{checkInErr: injected}
	svc := &service{ServiceDependencies: ServiceDependencies{AttendanceSyncer: syncer}}
	at := time.Now()
	previous := &studentpresence.Visit{StudentID: 1, ActiveGroupID: 2, EntryTime: at.Add(-time.Hour)}
	updated := &studentpresence.Visit{StudentID: 1, ActiveGroupID: 3, EntryTime: previous.EntryTime, ExitTime: &at}

	source, target, err := svc.syncMovedVisitAttendance(context.Background(), previous, updated, at)

	require.ErrorIs(t, err, injected)
	assert.Nil(t, source)
	assert.Nil(t, target)
	assert.Len(t, syncer.loaded, 1, "failed target check-in must stop before target checkout")
	assert.Len(t, syncer.mirrored, 1)
}

func TestGetVisitLookupErrorClassification(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("returns visit not found when lookup misses", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return nil, base.ErrNotFound
			},
		}}}

		visit, err := svc.GetVisit(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, visit)
		assert.True(t, errors.Is(err, ErrVisitNotFound), "expected ErrVisitNotFound")
	})

	t.Run("returns visit not found when lookup returns nil without error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return nil, nil
			},
		}}}

		visit, err := svc.GetVisit(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, visit)
		assert.True(t, errors.Is(err, ErrVisitNotFound), "expected ErrVisitNotFound")
	})

	t.Run("preserves database lookup failures", func(t *testing.T) {
		lookupErr := errors.New("visit query failed")
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return nil, lookupErr
			},
		}}}

		visit, err := svc.GetVisit(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, visit)
		assert.True(t, errors.Is(err, ErrDatabaseOperation), "expected ErrDatabaseOperation")
		assert.False(t, errors.Is(err, ErrVisitNotFound), "database failures must not masquerade as not found")
	})
}

func TestUpdateVisitPreloadAndTargetLookupErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	entryTime := time.Now()
	existingVisit := &studentpresence.Visit{
		ID:            100,
		StudentID:     200,
		ActiveGroupID: 300,
		EntryTime:     entryTime,
	}
	updatedVisit := &studentpresence.Visit{
		ID:            existingVisit.ID,
		StudentID:     existingVisit.StudentID,
		ActiveGroupID: 400,
		EntryTime:     entryTime,
	}

	t.Run("returns visit not found when preload misses", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return nil, base.ErrNotFound
			},
		}},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrVisitNotFound), "expected ErrVisitNotFound")
	})

	t.Run("returns visit not found when preload returns nil without error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return nil, nil
			},
		}},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrVisitNotFound), "expected ErrVisitNotFound")
	})

	t.Run("returns active group not found when target lookup misses", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return existingVisit, nil
			},
		}, GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return nil, base.ErrNotFound
			},
		}},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrActiveGroupNotFound), "expected ErrActiveGroupNotFound")
	})

	t.Run("preserves database errors from target lookup", func(t *testing.T) {
		lookupErr := errors.New("target lookup failed")
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return existingVisit, nil
			},
		}, GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return nil, lookupErr
			},
		}},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrDatabaseOperation), "expected ErrDatabaseOperation")
	})

	t.Run("returns active group not found when target lookup returns nil without error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return existingVisit, nil
			},
		}, GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return nil, nil
			},
		}},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrActiveGroupNotFound), "expected ErrActiveGroupNotFound")
	})

	t.Run("returns active group not found when target group is inactive", func(t *testing.T) {
		endTime := entryTime.Add(time.Hour)
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return existingVisit, nil
			},
		}, GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return &activeModels.Group{
					Model:   base.Model{ID: updatedVisit.ActiveGroupID},
					EndTime: &endTime,
				}, nil
			},
		}},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrActiveGroupNotFound), "expected ErrActiveGroupNotFound")
	})

	t.Run("updates without target lookup when active group does not change", func(t *testing.T) {
		updateCalled := false
		sameGroupVisit := &studentpresence.Visit{
			ID:            existingVisit.ID,
			StudentID:     existingVisit.StudentID,
			ActiveGroupID: existingVisit.ActiveGroupID,
			EntryTime:     entryTime,
		}
		svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return existingVisit, nil
			},
			updateFunc: func(context.Context, *studentpresence.Visit) error {
				updateCalled = true
				return nil
			},
		}, GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				t.Fatal("target group should not be loaded when active group is unchanged")
				return nil, nil
			},
		}},
		}

		err := svc.UpdateVisit(ctx, sameGroupVisit)

		require.NoError(t, err)
		assert.True(t, updateCalled, "expected visit update")
	})
}

func TestUpdateVisitLocksAttendanceBeforeClosingIt(t *testing.T) {
	t.Parallel()

	entryTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC).Add(-time.Hour)
	exitTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	existing := &studentpresence.Visit{
		ID: 101, StudentID: 201, ActiveGroupID: 301, EntryTime: entryTime,
	}
	updated := *existing
	updated.ExitTime = &exitTime
	attendance := &lockingAttendanceRepository{row: studentpresence.Attendance{
		StudentID: existing.StudentID, Date: timezone.DateFromTime(entryTime).String(), CheckInTime: entryTime,
	}}
	attendance.StudentPresence = &mockVisitRepository{
		findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
			return existing, nil
		},
	}
	svc := &service{ServiceDependencies: ServiceDependencies{
		SchoolPresence: attendance,
	}}

	err := svc.UpdateVisit(context.Background(), &updated)

	require.NoError(t, err)
	assert.Equal(t, []string{"lock", "find", "update"}, attendance.calls)
}

func TestUpdateVisitMoveSynchronizesSourceAndTargetWithoutBroadcaster(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	entryTime := time.Now().Add(-time.Hour)
	existingVisit := &studentpresence.Visit{
		ID:            100,
		StudentID:     200,
		ActiveGroupID: 300,
		EntryTime:     entryTime,
	}
	updatedVisit := &studentpresence.Visit{
		ID:            existingVisit.ID,
		StudentID:     existingVisit.StudentID,
		ActiveGroupID: 400,
		EntryTime:     entryTime,
	}
	syncer := &recordingAttendanceSyncer{}
	svc := &service{ServiceDependencies: ServiceDependencies{
		SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return existingVisit, nil
			},
			updateFunc: func(context.Context, *studentpresence.Visit) error { return nil },
		},
		GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return &activeModels.Group{Model: base.Model{ID: updatedVisit.ActiveGroupID}}, nil
			},
		},
		AttendanceSyncer: syncer,
	}}

	require.NoError(t, svc.UpdateVisit(ctx, updatedVisit))
	require.Len(t, syncer.loaded, 1)
	assert.Equal(t, existingVisit.ActiveGroupID, syncer.loaded[0].ActiveGroupID)
	require.NotNil(t, syncer.loaded[0].ExitTime)
	require.Len(t, syncer.mirrored, 1)
	assert.Equal(t, updatedVisit.ActiveGroupID, syncer.mirrored[0].ActiveGroupID)
	assert.Nil(t, syncer.mirrored[0].ExitTime)
	assert.WithinDuration(t, *syncer.loaded[0].ExitTime, syncer.mirrored[0].EntryTime, time.Millisecond)
}

func TestUpdateVisitCheckoutOnlySynchronizesSlotAttendance(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	entryTime := time.Now().Add(-time.Hour)
	exitTime := time.Now()
	existingVisit := &studentpresence.Visit{
		ID:            100,
		StudentID:     200,
		ActiveGroupID: 300,
		EntryTime:     entryTime,
	}
	updatedVisit := &studentpresence.Visit{
		ID:            existingVisit.ID,
		StudentID:     existingVisit.StudentID,
		ActiveGroupID: existingVisit.ActiveGroupID,
		EntryTime:     entryTime,
		ExitTime:      &exitTime,
	}
	syncer := &recordingAttendanceSyncer{}
	svc := &service{ServiceDependencies: ServiceDependencies{
		SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return existingVisit, nil
			},
			updateFunc: func(context.Context, *studentpresence.Visit) error { return nil },
		},
		AttendanceSyncer: syncer,
	}}

	require.NoError(t, svc.UpdateVisit(ctx, updatedVisit))
	require.Len(t, syncer.revised, 1, "checkout-only update must reconcile slot attendance")
	assert.Equal(t, existingVisit.EntryTime, syncer.revised[0][0].EntryTime)
	require.NotNil(t, syncer.revised[0][1].ExitTime)
	assert.Equal(t, exitTime, *syncer.revised[0][1].ExitTime)
	assert.Empty(t, syncer.loaded, "same-group edits use guarded interval reconciliation")
	assert.Empty(t, syncer.mirrored, "no group move, no check-in mirror")
}

func TestUpdateVisitOpenEntryTimeEditReconcilesSlot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	entryTime := time.Now().Add(-time.Hour)
	existingVisit := &studentpresence.Visit{
		ID:            100,
		StudentID:     200,
		ActiveGroupID: 300,
		EntryTime:     entryTime,
	}
	updatedVisit := &studentpresence.Visit{
		ID:            existingVisit.ID,
		StudentID:     existingVisit.StudentID,
		ActiveGroupID: existingVisit.ActiveGroupID,
		EntryTime:     entryTime.Add(-time.Minute),
	}
	syncer := &recordingAttendanceSyncer{}
	svc := &service{ServiceDependencies: ServiceDependencies{
		SchoolPresence: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
				return existingVisit, nil
			},
			updateFunc: func(context.Context, *studentpresence.Visit) error { return nil },
		},
		AttendanceSyncer: syncer,
	}}

	require.NoError(t, svc.UpdateVisit(ctx, updatedVisit))
	require.Len(t, syncer.revised, 1)
	assert.Equal(t, existingVisit.EntryTime, syncer.revised[0][0].EntryTime)
	assert.Equal(t, updatedVisit.EntryTime, syncer.revised[0][1].EntryTime)
	assert.Empty(t, syncer.mirrored)
}

func TestUpdateVisitClosedIntervalEditAndReopenReconcileSlot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	entryTime := time.Now().Add(-2 * time.Hour)
	exitTime := entryTime.Add(time.Hour)
	tests := []struct {
		name       string
		updatedOut *time.Time
	}{
		{name: "closed interval edit", updatedOut: func() *time.Time { v := exitTime.Add(15 * time.Minute); return &v }()},
		{name: "reopen", updatedOut: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existingVisit := &studentpresence.Visit{
				ID: 100, StudentID: 200, ActiveGroupID: 300,
				EntryTime: entryTime, ExitTime: &exitTime,
			}
			updatedVisit := &studentpresence.Visit{
				ID: existingVisit.ID, StudentID: existingVisit.StudentID,
				ActiveGroupID: existingVisit.ActiveGroupID, EntryTime: entryTime, ExitTime: tt.updatedOut,
			}
			syncer := &recordingAttendanceSyncer{}
			svc := &service{ServiceDependencies: ServiceDependencies{
				SchoolPresence: &mockVisitRepository{
					findByIDFunc: func(context.Context, interface{}) (*studentpresence.Visit, error) {
						return existingVisit, nil
					},
					updateFunc: func(context.Context, *studentpresence.Visit) error { return nil },
				},
				GroupRepo:        &mockGroupRepository{},
				AttendanceSyncer: syncer,
			}}

			require.NoError(t, svc.UpdateVisit(ctx, updatedVisit))
			require.Len(t, syncer.revised, 1)
			require.NotNil(t, syncer.revised[0][0].ExitTime)
			assert.Equal(t, exitTime, *syncer.revised[0][0].ExitTime)
			assert.Equal(t, tt.updatedOut, syncer.revised[0][1].ExitTime)
		})
	}
}
