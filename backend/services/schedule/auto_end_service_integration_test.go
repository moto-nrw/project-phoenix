package schedule_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var autoEndNow = time.Date(2026, 4, 20, 15, 15, 0, 0, timezone.Berlin)

func TestAutoEnd_UsesAtomicManualCompletionPath(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedInstance(t, s, true, true)
	started, err := s.svc.Start(s.ctx, instance.ID, s.staffID)
	require.NoError(t, err)

	visit := &activeModels.Visit{
		StudentID:     s.student1,
		ActiveGroupID: started.ActiveGroupID,
		EntryTime:     autoEndNow.Add(-time.Hour),
	}
	visit.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, s.repos.ActiveVisit.Create(s.ctx, visit))
	updated, err := s.repos.InstanceStudent.UpdateAttendanceFromCheckin(
		s.ctx, instance.ID, s.student1, visit.EntryTime,
	)
	require.NoError(t, err)
	require.True(t, updated)

	autoEnd := scheduleSvc.NewAutoEndService(s.repos.ActivityInstance, s.svc)
	var result *scheduleSvc.AutoEndResult
	err = testpkg.WithTenantTx(t, context.Background(), s.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var runErr error
		result, runErr = autoEnd.RunForTenant(txCtx, autoEndNow, 15*time.Minute)
		return runErr
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Completed)

	reloaded, err := s.repos.ActivityInstance.FindByID(s.ctx, instance.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCompleted, reloaded.Status)
	require.NotNil(t, reloaded.CompletedAt)

	group, err := s.factory.Active.GetActiveGroup(s.ctx, started.ActiveGroupID)
	require.NoError(t, err)
	require.NotNil(t, group.EndTime)

	endedVisit, err := s.factory.Active.GetVisit(s.ctx, visit.ID)
	require.NoError(t, err)
	require.NotNil(t, endedVisit.ExitTime)

	supervisors, err := s.repos.GroupSupervisor.FindByActiveGroupID(s.ctx, started.ActiveGroupID, false)
	require.NoError(t, err)
	require.Len(t, supervisors, 1)
	require.NotNil(t, supervisors[0].EndDate)

	attendance, err := s.repos.InstanceStudent.FindByInstanceAndStudent(s.ctx, instance.ID, s.student1)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, attendance.Status)
	require.NotNil(t, attendance.CheckedOutAt)
}

func TestAutoEnd_ConcurrentManualCompletionHasOneWinner(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := instanceServiceWithBroadcaster(s, broadcaster)
	instance := seedInstance(t, s, true, false)
	_, err := svc.Start(s.ctx, instance.ID, s.staffID)
	require.NoError(t, err)
	autoEnd := scheduleSvc.NewAutoEndService(s.repos.ActivityInstance, svc)

	start := make(chan struct{})
	manualErr := make(chan error, 1)
	autoResult := make(chan *scheduleSvc.AutoEndResult, 1)
	autoErr := make(chan error, 1)
	var ready sync.WaitGroup
	ready.Add(2)

	go func() {
		ready.Done()
		<-start
		manualErr <- testpkg.WithTenantTx(t, context.Background(), s.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			_, completeErr := svc.Complete(txCtx, instance.ID)
			return completeErr
		})
	}()
	go func() {
		ready.Done()
		<-start
		var result *scheduleSvc.AutoEndResult
		err := testpkg.WithTenantTx(t, context.Background(), s.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			var runErr error
			result, runErr = autoEnd.RunForTenant(txCtx, autoEndNow, 15*time.Minute)
			return runErr
		})
		autoResult <- result
		autoErr <- err
	}()

	ready.Wait()
	close(start)
	manualCompletionErr := <-manualErr
	automatic := <-autoResult
	require.NoError(t, <-autoErr)
	require.NotNil(t, automatic)

	winners := automatic.Completed
	if manualCompletionErr == nil {
		winners++
	} else {
		assert.True(t, errors.Is(manualCompletionErr, scheduleSvc.ErrInvalidInstanceTransition))
	}
	assert.Equal(t, 1, winners)
	assert.Equal(t, 1-automatic.Completed, automatic.SkippedConcurrent)

	var repeated *scheduleSvc.AutoEndResult
	err = testpkg.WithTenantTx(t, context.Background(), s.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var runErr error
		repeated, runErr = autoEnd.RunForTenant(txCtx, autoEndNow, 15*time.Minute)
		return runErr
	})
	require.NoError(t, err)
	require.NotNil(t, repeated)
	assert.Zero(t, repeated.Completed)

	require.Eventually(t, func() bool {
		completedEvents := 0
		for _, call := range broadcaster.CallsByMethod("tenant") {
			if call.Event.Type == realtime.EventInstanceCompleted {
				completedEvents++
			}
		}
		return completedEvents == 1
	}, time.Second, 10*time.Millisecond, "parallel completion must emit one completion event")
}

func TestAutoEnd_IsTenantIsolated(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedInstance(t, s, true, false)
	_, err := s.svc.Start(s.ctx, instance.ID, s.staffID)
	require.NoError(t, err)

	foreign := testpkg.NewTenantScope(t, s.db)
	foreignCtx := foreign.Context()
	foreignRoom := &facilitiesModels.Room{Name: "Foreign tenant room"}
	foreignRoom.SetTenantID(foreign.TenantID)
	require.NoError(t, s.repos.Room.Create(foreignCtx, foreignRoom))
	foreignGroup := &activeModels.Group{
		StartTime:    autoEndNow.Add(-time.Hour),
		LastActivity: autoEndNow.Add(-time.Minute),
		RoomID:       foreignRoom.ID,
	}
	foreignGroup.SetTenantID(foreign.TenantID)
	require.NoError(t, s.repos.ActiveGroup.Create(foreignCtx, foreignGroup))

	foreignInstance := &scheduleModels.ActivityInstance{
		Date:          scheduleModels.DateFromTime(autoEndNow),
		Title:         "Foreign tenant active instance",
		StartTime:     time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:       time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:        foreignRoom.ID,
		Status:        scheduleModels.InstanceStatusActive,
		ActiveGroupID: &foreignGroup.ID,
	}
	foreignInstance.SetTenantID(foreign.TenantID)
	require.NoError(t, s.repos.ActivityInstance.Create(foreignCtx, foreignInstance))

	autoEnd := scheduleSvc.NewAutoEndService(s.repos.ActivityInstance, s.svc)
	var result *scheduleSvc.AutoEndResult
	err = testpkg.WithTenantTx(t, context.Background(), s.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var runErr error
		result, runErr = autoEnd.RunForTenant(txCtx, autoEndNow, 15*time.Minute)
		return runErr
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Checked)
	assert.Equal(t, 1, result.Completed)

	untouched, err := s.repos.ActivityInstance.FindByID(foreignCtx, foreignInstance.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusActive, untouched.Status)
}
