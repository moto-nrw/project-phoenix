package httpintegration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The single delete of a block (#6499) shares the day-lock protocol of
// Start and UpdatePlanned: it re-reads the block under its day lock and
// decides on that state. Each race runs the real lifecycle commands in two
// transactions; a parked command waits at a channel, never on a timer.

// A start that commits after the delete's unlocked read turns the block into
// history. Without the locked re-read the delete removed it, and the
// CASCADE took the session and its attendance along.
func TestDeleteCancelled_RefusesBlockStartedAfterItsRead(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)
	deleteDone, release := parkedDelete(t, s, ai.ID)

	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err, "start must commit while the delete is parked after its read")
	release()

	deleteErr := awaitDelete(t, s, deleteDone)
	require.ErrorIs(t, deleteErr, timetable.ErrInvalidInstanceTransition, "the delete must decide on the started block")
	assertStartedBlockIntact(t, s, ai.ID, started.ActiveGroupID, 2)
}

// A delete that arrives while a start holds the day lock waits for it and
// then refuses the started block. Without the day lock the delete committed
// under the running start.
func TestDeleteCancelled_WaitsForRunningStart(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)
	parked := make(chan int, 1)
	releaseStart := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(releaseStart) }) }
	t.Cleanup(release)
	deps := lifecycleDependencies(s, nil)
	deps.StartConflicts = pausingStartConflicts{StartConflictQuery: deps.StartConflicts, after: func(ctx context.Context) error {
		parked <- backendPID(t, ctx)
		select {
		case <-releaseStart:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	starter := newLifecycle(t, deps)
	type startOutcome struct {
		result *timetable.StartInstanceResult
		err    error
	}
	startDone := make(chan startOutcome, 1)
	go func() {
		var outcome startOutcome
		outcome.err = testpkg.WithTenantTx(t, s.ctx, s.db, tenant.FromContext(s.ctx), func(ctx context.Context, _ bun.Tx) error {
			var err error
			outcome.result, err = starter.Start(ctx, ai.ID, 0)
			return err
		})
		startDone <- outcome
	}()
	startPID := <-parked

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- deleteInTenantTx(t, s.ctx, s.db, s.svc, ai.ID)
	}()
	awaitBlockedBy(t, s, startPID, deleteDone)
	release()

	started := <-startDone
	require.NoError(t, started.err)
	require.ErrorIs(t, awaitDelete(t, s, deleteDone), timetable.ErrInvalidInstanceTransition)
	assertStartedBlockIntact(t, s, ai.ID, started.result.ActiveGroupID, 2)
}

// After a committed delete, a start that loaded the block before it finds
// nothing under its day lock: no session, no room group, no success.
func TestStart_RefusesBlockDeletedAfterItsRead(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)
	parked := make(chan struct{})
	releaseStart := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(releaseStart) }) }
	t.Cleanup(release)
	deps := lifecycleDependencies(s, nil)
	deps.InstanceRepo.Reads = fixtures.AfterFirstInstanceRead(deps.InstanceRepo.Reads, ai.ID, func(ctx context.Context) error {
		close(parked)
		select {
		case <-releaseStart:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	starter := newLifecycle(t, deps)
	startDone := make(chan error, 1)
	go func() {
		startDone <- testpkg.WithTenantTx(t, s.ctx, s.db, tenant.FromContext(s.ctx), func(ctx context.Context, _ bun.Tx) error {
			_, err := starter.Start(ctx, ai.ID, 0)
			return err
		})
	}()
	<-parked

	require.NoError(t, deleteInTenantTx(t, s.ctx, s.db, s.svc, ai.ID))
	release()

	require.ErrorIs(t, <-startDone, timetable.ErrInstanceNotFound)
	assert.False(t, instanceExists(t, s, ai.ID))
	assert.Zero(t, countRowsWhere(t, s, "active.groups", "room_id", s.roomID), "a refused start must not leave a room group behind")
}

// A move that commits after the delete's read changes the slot whose
// exception the delete writes. Without the locked re-read the delete removed
// the moved block and consumed only the old slot, which the move had
// already consumed.
func TestDeleteCancelled_RefusesBlockMovedAfterItsRead(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	ai := seedInstance(t, s, false, false)
	deleteDone, release := parkedDelete(t, s, ai.ID)
	from, to := calendar.Date(ai.Date), calendar.Date(ai.Date).AddDays(1)

	_, err := s.svc.UpdatePlanned(s.ctx, ai.ID, timetable.UpdateInstanceInput{
		Date:            to,
		StartTime:       time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:         time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		Title:           ai.Title,
		RoomID:          s.roomID,
		ActivityGroupID: &s.tmplID,
	}, nil)
	require.NoError(t, err, "the move must commit while the delete is parked after its read")
	release()

	require.ErrorIs(t, awaitDelete(t, s, deleteDone), timetable.ErrInstanceMoved)
	require.True(t, instanceExists(t, s, ai.ID), "the moved block stays")
	assert.Equal(t, to, calendar.Date(reloadInstance(t, s, ai.ID).Date))
	exceptions := loadLifecycleExceptions(t, s)
	require.Len(t, exceptions, 1, "only the move's exception for the vacated slot")
	assert.Equal(t, from, calendar.Date(exceptions[0].ExceptionDate))
	require.NotNil(t, exceptions[0].Reason)
	assert.Equal(t, "Einzeltermin verschoben", *exceptions[0].Reason)
}

// parkedDelete runs DeleteCancelled of the block in its own transaction and
// parks it right after its first read of the block, until release.
func parkedDelete(t *testing.T, s *lifecycleSetup, instanceID int64) (<-chan error, func()) {
	t.Helper()
	parked := make(chan struct{})
	releaseDelete := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(releaseDelete) }) }
	t.Cleanup(release)
	deps := lifecycleDependencies(s, nil)
	deps.InstanceRepo.Reads = fixtures.AfterFirstInstanceRead(deps.InstanceRepo.Reads, instanceID, func(ctx context.Context) error {
		close(parked)
		select {
		case <-releaseDelete:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	deleter := newLifecycle(t, deps)
	done := make(chan error, 1)
	go func() {
		done <- deleteInTenantTx(t, s.ctx, s.db, deleter, instanceID)
	}()
	select {
	case <-parked:
	case err := <-done:
		t.Fatalf("delete finished before its first read of the block: %v", err)
	}
	return done, release
}

func awaitDelete(t *testing.T, s *lifecycleSetup, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-s.ctx.Done():
		t.Fatal("delete did not finish: ", s.ctx.Err())
		return nil
	}
}

// awaitBlockedBy returns once PostgreSQL reports a lock request blocked by
// the holder. The ticker only paces observation; the go test timeout bounds
// a command that never queues (#5221/#5271).
func awaitBlockedBy(t *testing.T, s *lifecycleSetup, holderPID int, done <-chan error) {
	t.Helper()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		var waiting bool
		require.NoError(t, s.db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks l
   WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))`, holderPID).Scan(s.ctx, &waiting))
		if waiting {
			return
		}
		select {
		case err := <-done:
			t.Fatalf("delete bypassed the running start: %v", err)
		case <-poll.C:
		}
	}
}

func backendPID(t *testing.T, ctx context.Context) int {
	t.Helper()
	txValue, ok := tenant.TransactionFromContext(ctx)
	require.True(t, ok)
	var pid int
	require.NoError(t, txValue.(bun.Tx).NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid))
	return pid
}

// assertStartedBlockIntact checks that the started block kept its row, its
// participants and its open room group.
func assertStartedBlockIntact(t *testing.T, s *lifecycleSetup, instanceID, activeGroupID int64, participants int) {
	t.Helper()
	require.True(t, instanceExists(t, s, instanceID), "the started block stays")
	assert.Equal(t, fixtures.InstanceStatusActive(), reloadInstance(t, s, instanceID).Status)
	assert.Equal(t, participants, countRowsWhere(t, s, "schedule.instance_students", "instance_id", instanceID))
	var open int
	require.NoError(t, s.db.NewRaw("SELECT count(*) FROM active.groups WHERE tenant_id = ? AND id = ? AND end_time IS NULL",
		tenant.FromContext(s.ctx), activeGroupID).Scan(s.ctx, &open))
	assert.Equal(t, 1, open, "the room group of the started block stays open")
}

type pausingStartConflicts struct {
	timetable.StartConflictQuery
	after func(context.Context) error
}

func (p pausingStartConflicts) DetectStartConflicts(ctx context.Context, subject timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error) {
	if err := p.after(ctx); err != nil {
		return nil, err
	}
	return p.StartConflictQuery.DetectStartConflicts(ctx, subject)
}

// deleteInTenantTx runs DeleteCancelled in the tenant transaction its route
// opens: the delete holds the block's day lock until commit.
func deleteInTenantTx(t *testing.T, ctx context.Context, db *bun.DB, svc interface {
	DeleteCancelled(context.Context, int64) error
}, instanceID int64) error {
	t.Helper()
	return testpkg.WithTenantTx(t, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
		return svc.DeleteCancelled(txCtx, instanceID)
	})
}
