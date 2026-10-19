package httpintegration_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var autoEndNow = time.Date(2026, 4, 20, 15, 15, 0, 0, calendar.Berlin)

func TestAutoEnd_UsesAtomicManualCompletionPath(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedInstance(t, s, true, true)
	started, err := s.svc.Start(s.ctx, instance.ID, s.staffID)
	require.NoError(t, err)

	visit := testpkg.CreateTestVisit(t, s.db, s.student1, started.ActiveGroupID, autoEndNow.Add(-time.Hour), nil)
	updated, err := fixtures.FromInstanceStudentRecords(s.timetable.InstanceStudent).UpdateAttendanceFromCheckin(
		s.ctx, instance.ID, s.student1, visit.EntryTime,
	)
	require.NoError(t, err)
	require.True(t, updated)

	autoEnd := newLifecycleAutoEnd(t, fixtures.FromActivityInstanceRecords(s.timetable.ActivityInstance), s.svc)
	var result *timetable.AutoEndResult
	err = testpkg.WithTenantTx(t, context.Background(), s.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var runErr error
		result, runErr = autoEnd.RunForTenant(txCtx, autoEndNow, 15*time.Minute)
		return runErr
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Completed)

	reloaded, err := fixtures.FromActivityInstanceRecords(s.timetable.ActivityInstance).FindByID(s.ctx, instance.ID)
	require.NoError(t, err)
	assert.Equal(t, fixtures.InstanceStatusCompleted(), reloaded.Status)
	require.NotNil(t, reloaded.CompletedAt)

	group, err := s.active.GetActiveGroup(s.ctx, started.ActiveGroupID)
	require.NoError(t, err)
	require.NotNil(t, group.EndTime)

	presence := newInstancePresence(t, s.db)
	endedVisit, err := presence.FindVisit(s.ctx, visit.ID)
	require.NoError(t, err)
	require.NotNil(t, endedVisit.ExitTime)

	supervisors, err := s.repos.GroupSupervisor.FindByActiveGroupID(s.ctx, started.ActiveGroupID, false)
	require.NoError(t, err)
	require.Len(t, supervisors, 1)
	require.NotNil(t, supervisors[0].EndDate)

	attendance, err := fixtures.FromInstanceStudentRecords(s.timetable.InstanceStudent).FindByInstanceAndStudent(s.ctx, instance.ID, s.student1)
	require.NoError(t, err)
	assert.Equal(t, fixtures.AttendanceStatusPresent(), attendance.Status)
	require.NotNil(t, attendance.CheckedOutAt)
}

func TestAutoEnd_ConcurrentManualCompletionHasOneWinner(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := instanceServiceWithBroadcaster(t, s, broadcaster)
	instance := seedInstance(t, s, true, false)
	_, err := svc.Start(s.ctx, instance.ID, s.staffID)
	require.NoError(t, err)
	autoEnd := newLifecycleAutoEnd(t, fixtures.FromActivityInstanceRecords(s.timetable.ActivityInstance), svc)
	start := make(chan struct{})
	manualErr := make(chan error, 1)
	autoResult := make(chan *timetable.AutoEndResult, 1)
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
		var result *timetable.AutoEndResult
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
		assert.Zero(t, automatic.Completed)
	} else {
		assert.True(t, errors.Is(manualCompletionErr, timetable.ErrInvalidInstanceTransition))
		assert.Equal(t, 1, automatic.Completed)
		assert.Zero(t, automatic.SkippedConcurrent)
	}
	assert.Equal(t, 1, winners)
	// A manual completion can win before the scheduler lists active instances.
	// In that valid ordering the scheduler has no concurrent completion to skip.
	assert.LessOrEqual(t, automatic.SkippedConcurrent, 1)

	var repeated *timetable.AutoEndResult
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
	foreignRoom := testpkg.CreateTestRoomForTenant(t, s.db, foreign.TenantID, "Foreign tenant room")
	foreignGroup := &studentpresence.LiveGroup{
		StartTime:    autoEndNow.Add(-time.Hour),
		LastActivity: autoEndNow.Add(-time.Minute),
		RoomID:       foreignRoom.ID,
		TenantID:     foreign.TenantID,
	}
	require.NoError(t, s.repos.ActiveGroup.CreateSession(foreignCtx, foreignGroup))

	foreignInstance := fixtures.NewActivityInstance(func(fixtureRow *fixtures.ActivityInstance) {
		fixtureRow.Date = calendar.DateFromTime(autoEndNow)
		fixtureRow.Title = "Foreign tenant active instance"
		fixtureRow.StartTime = time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)
		fixtureRow.EndTime = time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)
		fixtureRow.RoomID = foreignRoom.ID
		fixtureRow.Status = fixtures.InstanceStatusActive()
		fixtureRow.ActiveGroupID = &foreignGroup.ID
	})

	foreignInstance.SetTenantID(foreign.TenantID)
	require.NoError(t, fixtures.FromActivityInstanceRecords(s.timetable.ActivityInstance).Create(foreignCtx, foreignInstance))

	autoEnd := newLifecycleAutoEnd(t, fixtures.FromActivityInstanceRecords(s.timetable.ActivityInstance), s.svc)
	var result *timetable.AutoEndResult
	err = testpkg.WithTenantTx(t, context.Background(), s.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var runErr error
		result, runErr = autoEnd.RunForTenant(txCtx, autoEndNow, 15*time.Minute)
		return runErr
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Checked)
	assert.Equal(t, 1, result.Completed)

	untouched, err := fixtures.FromActivityInstanceRecords(s.timetable.ActivityInstance).FindByID(foreignCtx, foreignInstance.ID)
	require.NoError(t, err)
	assert.Equal(t, fixtures.InstanceStatusActive(), untouched.Status)
}

// TestAutoEnd_LeavesIndependentRoomStays pins the #3066 lifecycle for the
// automatic end: completing a due activity ends only its own session. A child
// who stays in the same released room without the activity (the room's own
// device-less session under a system activity) keeps that stay.
func TestAutoEnd_LeavesIndependentRoomStays(t *testing.T) {
	t.Parallel()

	s := buildLifecycle(t)
	instance := seedInstance(t, s, true, true)
	started, err := s.svc.Start(s.ctx, instance.ID, s.staffID)
	require.NoError(t, err)
	participant := testpkg.CreateTestVisit(t, s.db, s.student1, started.ActiveGroupID, autoEndNow.Add(-time.Hour), nil)

	roomActivity := testpkg.CreateTestActivityGroup(t, s.db, fmt.Sprintf("AE-Offener-Raum-%d", time.Now().UnixNano()))
	_, err = s.db.NewUpdate().
		Table("activities.groups").
		Set("is_system = TRUE").
		Where("id = ?", roomActivity.ID).
		Exec(s.ctx)
	require.NoError(t, err)
	roomSession := testpkg.CreateTestActiveGroup(t, s.db, roomActivity.ID, s.roomID)
	independent := testpkg.CreateTestVisit(t, s.db, s.student2, roomSession.ID, autoEndNow.Add(-30*time.Minute), nil)

	autoEnd := newLifecycleAutoEnd(t, fixtures.FromActivityInstanceRecords(s.timetable.ActivityInstance), s.svc)
	var result *timetable.AutoEndResult
	err = testpkg.WithTenantTx(t, context.Background(), s.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var runErr error
		result, runErr = autoEnd.RunForTenant(txCtx, autoEndNow, 15*time.Minute)
		return runErr
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Completed)

	presence := newInstancePresence(t, s.db)
	endedVisit, err := presence.FindVisit(s.ctx, participant.ID)
	require.NoError(t, err)
	assert.NotNil(t, endedVisit.ExitTime, "the activity's child leaves with the activity")

	keptVisit, err := presence.FindVisit(s.ctx, independent.ID)
	require.NoError(t, err)
	assert.Nil(t, keptVisit.ExitTime, "the independent stay in the same room continues")

	keptSession, err := s.active.GetActiveGroup(s.ctx, roomSession.ID)
	require.NoError(t, err)
	assert.Nil(t, keptSession.EndTime, "the room's own session is not the activity's session")
}
