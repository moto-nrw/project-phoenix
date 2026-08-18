package active

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lockingAttendanceRepository struct {
	activeModels.AttendanceRepository
	calls []string
	row   *activeModels.Attendance
}

func (r *lockingAttendanceRepository) LockStudentAttendance(context.Context, int64) error {
	r.calls = append(r.calls, "lock")
	return nil
}

func (r *lockingAttendanceRepository) FindByStudentAndDate(context.Context, int64, timezone.Date) ([]*activeModels.Attendance, error) {
	r.calls = append(r.calls, "find")
	return []*activeModels.Attendance{r.row}, nil
}

func (r *lockingAttendanceRepository) Update(context.Context, *activeModels.Attendance) error {
	r.calls = append(r.calls, "update")
	return nil
}

type recordingAttendanceSyncer struct {
	loaded   []*activeModels.Visit
	mirrored []*activeModels.Visit
	revised  [][2]*activeModels.Visit
	mirrorAt []struct {
		studentID int64
		at        time.Time
	}
}

func (r *recordingAttendanceSyncer) MirrorCheckInForVisit(_ context.Context, visit *activeModels.Visit) *AttendanceSnapshot {
	copy := *visit
	r.mirrored = append(r.mirrored, &copy)
	return &AttendanceSnapshot{Status: "present", InstanceID: 2}
}

func (r *recordingAttendanceSyncer) MirrorCheckInAt(_ context.Context, studentID int64, at time.Time) *AttendanceSnapshot {
	r.mirrorAt = append(r.mirrorAt, struct {
		studentID int64
		at        time.Time
	}{studentID: studentID, at: at})
	return nil
}

func (r *recordingAttendanceSyncer) MirrorCheckOutForVisit(_ context.Context, visit *activeModels.Visit) *AttendanceSnapshot {
	copy := *visit
	r.loaded = append(r.loaded, &copy)
	return &AttendanceSnapshot{Status: "present", InstanceID: 1}
}

func (r *recordingAttendanceSyncer) MirrorVisitRevision(_ context.Context, previous, updated *activeModels.Visit) {
	previousCopy := *previous
	updatedCopy := *updated
	r.revised = append(r.revised, [2]*activeModels.Visit{&previousCopy, &updatedCopy})
}

func (r *recordingAttendanceSyncer) MirrorCheckOutAt(context.Context, int64, time.Time) {}

func (r *recordingAttendanceSyncer) MirrorCheckInAtBatch(context.Context, []int64, time.Time) {}

func (r *recordingAttendanceSyncer) MirrorCheckOutAtBatch(context.Context, []int64, time.Time) {}

func (r *recordingAttendanceSyncer) MirrorCheckOutForVisits(_ context.Context, visits []*activeModels.Visit, _ time.Time) {
	for _, visit := range visits {
		copy := *visit
		r.loaded = append(r.loaded, &copy)
	}
}

func TestGetVisitLookupErrorClassification(t *testing.T) {
	ctx := context.Background()

	t.Run("returns visit not found when lookup misses", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
				return nil, sql.ErrNoRows
			},
		}}}

		visit, err := svc.GetVisit(ctx, 100)

		require.Error(t, err)
		assert.Nil(t, visit)
		assert.True(t, errors.Is(err, ErrVisitNotFound), "expected ErrVisitNotFound")
	})

	t.Run("returns visit not found when lookup returns nil without error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
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
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
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
	ctx := context.Background()
	entryTime := time.Now()
	existingVisit := &activeModels.Visit{
		Model:         base.Model{ID: 100},
		StudentID:     200,
		ActiveGroupID: 300,
		EntryTime:     entryTime,
	}
	updatedVisit := &activeModels.Visit{
		Model:         base.Model{ID: existingVisit.ID},
		StudentID:     existingVisit.StudentID,
		ActiveGroupID: 400,
		EntryTime:     entryTime,
	}

	t.Run("returns visit not found when preload misses", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
				return nil, sql.ErrNoRows
			},
		}},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrVisitNotFound), "expected ErrVisitNotFound")
	})

	t.Run("returns visit not found when preload returns nil without error", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
				return nil, nil
			},
		}},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrVisitNotFound), "expected ErrVisitNotFound")
	})

	t.Run("returns active group not found when target lookup misses", func(t *testing.T) {
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
				return existingVisit, nil
			},
		}, GroupRepo: &mockGroupRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Group, error) {
				return nil, sql.ErrNoRows
			},
		}},
		}

		err := svc.UpdateVisit(ctx, updatedVisit)

		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrActiveGroupNotFound), "expected ErrActiveGroupNotFound")
	})

	t.Run("preserves database errors from target lookup", func(t *testing.T) {
		lookupErr := errors.New("target lookup failed")
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
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
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
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
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
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
		sameGroupVisit := &activeModels.Visit{
			Model:         base.Model{ID: existingVisit.ID},
			StudentID:     existingVisit.StudentID,
			ActiveGroupID: existingVisit.ActiveGroupID,
			EntryTime:     entryTime,
		}
		svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
				return existingVisit, nil
			},
			updateFunc: func(context.Context, *activeModels.Visit) error {
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
	entryTime := time.Now().Add(-time.Hour)
	exitTime := time.Now()
	existing := &activeModels.Visit{
		Model: base.Model{ID: 101}, StudentID: 201, ActiveGroupID: 301, EntryTime: entryTime,
	}
	updated := *existing
	updated.ExitTime = &exitTime
	attendance := &lockingAttendanceRepository{row: &activeModels.Attendance{
		StudentID: existing.StudentID, Date: timezone.DateFromTime(entryTime), CheckInTime: entryTime,
	}}
	svc := &service{ServiceDependencies: ServiceDependencies{
		VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) { return existing, nil },
		},
		AttendanceRepo: attendance,
	}}

	err := svc.UpdateVisit(context.Background(), &updated)

	require.NoError(t, err)
	assert.Equal(t, []string{"lock", "find", "update"}, attendance.calls)
}

func TestUpdateVisitMoveSynchronizesSourceAndTargetWithoutBroadcaster(t *testing.T) {
	ctx := context.Background()
	entryTime := time.Now().Add(-time.Hour)
	existingVisit := &activeModels.Visit{
		Model:         base.Model{ID: 100},
		StudentID:     200,
		ActiveGroupID: 300,
		EntryTime:     entryTime,
	}
	updatedVisit := &activeModels.Visit{
		Model:         base.Model{ID: existingVisit.ID},
		StudentID:     existingVisit.StudentID,
		ActiveGroupID: 400,
		EntryTime:     entryTime,
	}
	syncer := &recordingAttendanceSyncer{}
	svc := &service{ServiceDependencies: ServiceDependencies{
		VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
				return existingVisit, nil
			},
			updateFunc: func(context.Context, *activeModels.Visit) error { return nil },
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
	ctx := context.Background()
	entryTime := time.Now().Add(-time.Hour)
	exitTime := time.Now()
	existingVisit := &activeModels.Visit{
		Model:         base.Model{ID: 100},
		StudentID:     200,
		ActiveGroupID: 300,
		EntryTime:     entryTime,
	}
	updatedVisit := &activeModels.Visit{
		Model:         base.Model{ID: existingVisit.ID},
		StudentID:     existingVisit.StudentID,
		ActiveGroupID: existingVisit.ActiveGroupID,
		EntryTime:     entryTime,
		ExitTime:      &exitTime,
	}
	syncer := &recordingAttendanceSyncer{}
	svc := &service{ServiceDependencies: ServiceDependencies{
		VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
				return existingVisit, nil
			},
			updateFunc: func(context.Context, *activeModels.Visit) error { return nil },
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
	ctx := context.Background()
	entryTime := time.Now().Add(-time.Hour)
	existingVisit := &activeModels.Visit{
		Model:         base.Model{ID: 100},
		StudentID:     200,
		ActiveGroupID: 300,
		EntryTime:     entryTime,
	}
	updatedVisit := &activeModels.Visit{
		Model:         base.Model{ID: existingVisit.ID},
		StudentID:     existingVisit.StudentID,
		ActiveGroupID: existingVisit.ActiveGroupID,
		EntryTime:     entryTime.Add(-time.Minute),
	}
	syncer := &recordingAttendanceSyncer{}
	svc := &service{ServiceDependencies: ServiceDependencies{
		VisitRepo: &mockVisitRepository{
			findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) {
				return existingVisit, nil
			},
			updateFunc: func(context.Context, *activeModels.Visit) error { return nil },
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
			existingVisit := &activeModels.Visit{
				Model: base.Model{ID: 100}, StudentID: 200, ActiveGroupID: 300,
				EntryTime: entryTime, ExitTime: &exitTime,
			}
			updatedVisit := &activeModels.Visit{
				Model: base.Model{ID: existingVisit.ID}, StudentID: existingVisit.StudentID,
				ActiveGroupID: existingVisit.ActiveGroupID, EntryTime: entryTime, ExitTime: tt.updatedOut,
			}
			syncer := &recordingAttendanceSyncer{}
			svc := &service{ServiceDependencies: ServiceDependencies{
				VisitRepo: &mockVisitRepository{
					findByIDFunc: func(context.Context, interface{}) (*activeModels.Visit, error) { return existingVisit, nil },
					updateFunc:   func(context.Context, *activeModels.Visit) error { return nil },
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
