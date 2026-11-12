package httpintegration_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	activityContract "github.com/moto-ogs/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	fixtures "github.com/moto-ogs/project-phoenix/modules/timetable/fixturetest"
	timetablePorts "github.com/moto-ogs/project-phoenix/modules/timetable/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The single delete of a block (#6499) shares the lock protocol of Start,
// UpdatePlanned and the materialization: the recurrence gate, then the
// block's day lock, and a decision on the block re-read under them. Each
// race runs the real commands in two transactions; a parked command waits
// at a channel, never on a timer.

// A start that commits after the delete's unlocked read turns the block into
// history. Without the locked re-read the delete removed it, and the
// CASCADE took the session and its attendance along.
func TestDeleteCancelled_RefusesBlockStartedAfterItsRead(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)
	point := newParkPoint(t)
	deleter := parkedLifecycle(t, s, ai.ID, point)
	deleteDone := runInTenantTx(t, s.ctx, s.db, func(ctx context.Context) error {
		return deleter.DeleteCancelled(ctx, ai.ID)
	})
	point.await(deleteDone)

	started, err := s.svc.Start(s.ctx, ai.ID, 0)
	require.NoError(t, err, "start must commit while the delete is parked after its read")
	point.release()

	require.ErrorIs(t, <-deleteDone, timetable.ErrInvalidInstanceTransition, "the delete must decide on the started block")
	assertStartedBlockIntact(t, s, ai.ID, started.ActiveGroupID, 2)
}

// A delete that arrives while a start holds the day lock waits for it and
// then refuses the started block. Without the day lock the delete committed
// under the running start.
func TestDeleteCancelled_WaitsForRunningStart(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)
	point := newParkPoint(t)
	deps := lifecycleDependencies(s, nil)
	deps.StartConflicts = parkingStartConflicts{StartConflictQuery: deps.StartConflicts, before: point.park}
	starter := newLifecycle(t, deps)
	var started *timetable.StartInstanceResult
	startDone := runInTenantTx(t, s.ctx, s.db, func(ctx context.Context) error {
		var err error
		started, err = starter.Start(ctx, ai.ID, 0)
		return err
	})
	startPID := point.await(startDone)

	deleteDone := runInTenantTx(t, s.ctx, s.db, func(ctx context.Context) error {
		return s.svc.DeleteCancelled(ctx, ai.ID)
	})
	if finished, err := awaitBlockedOrDone(t, s.ctx, s.db, startPID, deleteDone); finished {
		t.Fatalf("delete bypassed the running start: %v", err)
	}
	point.release()

	require.NoError(t, <-startDone)
	require.ErrorIs(t, <-deleteDone, timetable.ErrInvalidInstanceTransition)
	assertStartedBlockIntact(t, s, ai.ID, started.ActiveGroupID, 2)
}

// After a committed delete, a start that loaded the block before it finds
// nothing under its day lock: no session, no room group, no success.
func TestStart_RefusesBlockDeletedAfterItsRead(t *testing.T) {
	t.Parallel()
	s := buildLifecycle(t)
	ai := seedInstance(t, s, true, true)
	point := newParkPoint(t)
	starter := parkedLifecycle(t, s, ai.ID, point)
	startDone := runInTenantTx(t, s.ctx, s.db, func(ctx context.Context) error {
		_, err := starter.Start(ctx, ai.ID, 0)
		return err
	})
	point.await(startDone)

	require.NoError(t, deleteInTenantTx(t, s.ctx, s.db, s.svc, ai.ID))
	point.release()

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
	point := newParkPoint(t)
	deleter := parkedLifecycle(t, s, ai.ID, point)
	deleteDone := runInTenantTx(t, s.ctx, s.db, func(ctx context.Context) error {
		return deleter.DeleteCancelled(ctx, ai.ID)
	})
	point.await(deleteDone)
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
	point.release()

	require.ErrorIs(t, <-deleteDone, timetable.ErrInstanceMoved)
	require.True(t, instanceExists(t, s, ai.ID), "the moved block stays")
	assert.Equal(t, to, calendar.Date(reloadInstance(t, s, ai.ID).Date))
	exceptions := loadLifecycleExceptions(t, s)
	require.Len(t, exceptions, 1, "only the move's exception for the vacated slot")
	assert.Equal(t, from, calendar.Date(exceptions[0].ExceptionDate))
	require.NotNil(t, exceptions[0].Reason)
	assert.Equal(t, "Einzeltermin verschoben", *exceptions[0].Reason)
}

// A materialization loads the exceptions before the existing blocks. A
// delete that committed between the two loads left neither its exception
// nor its block in the run's view, so the run recreated the deleted
// occurrence. The delete and the bulk cancellation take the recurrence gate
// the run holds, wait for it, and the slot stays consumed.
func TestDeleteWaitsForRunningMaterialization(t *testing.T) {
	t.Parallel()
	removals := []struct {
		name   string
		remove func(context.Context, *scenarioSetup, int64, calendar.Date) error
	}{
		{"single delete", func(ctx context.Context, s *scenarioSetup, id int64, _ calendar.Date) error {
			return s.instance.DeleteCancelled(ctx, id)
		}},
		{"bulk cancellation", func(ctx context.Context, s *scenarioSetup, _ int64, date calendar.Date) error {
			_, err := s.instance.BulkCancelPlanned(ctx, date, date, timetable.BulkCancelOptions{}, nil)
			return err
		}},
	}
	for _, removal := range removals {
		t.Run(removal.name, func(t *testing.T) {
			date := calendar.NewDate(2026, time.August, 31) // a Monday after the scenario clock
			s := makeScenario(t, activityContract.WeekdayMonday, date)
			defer s.runCleanup(t)
			_, err := s.svc.MaterializeForTenant(s.ctx, date, date, timetable.MaterializationSourceManual)
			require.NoError(t, err)
			rows := listInstancesForDate(t, s.db, s.template.ID, date)
			require.Len(t, rows, 1)

			point := newParkPoint(t)
			materializer := newTestMaterialization(t, s.db,
				api.NewCareScheduleSuiteRepositories(testpkg.NewDatabase(t, s.db)),
				api.NewUnobservedTimetableRepositories(testpkg.MustDatabase(s.db)),
				func(deps *timetablePorts.MaterializationDependencies) {
					deps.ExceptionRepo = parkingExceptions{ActivityExceptionQuery: deps.ExceptionRepo, point: point}
				})
			runDone := runInTenantTx(t, s.ctx, s.db, func(ctx context.Context) error {
				_, err := materializer.MaterializeForTenant(ctx, date, date, timetable.MaterializationSourceManual)
				return err
			})
			runPID := point.await(runDone)

			removeDone := runInTenantTx(t, s.ctx, s.db, func(ctx context.Context) error {
				return removal.remove(ctx, s, rows[0].ID, date)
			})
			finished, removeErr := awaitBlockedOrDone(t, s.ctx, s.db, runPID, removeDone)
			point.release()
			require.NoError(t, <-runDone)
			if !finished {
				removeErr = <-removeDone
			}
			require.NoError(t, removeErr)

			assert.Empty(t, listInstancesForDate(t, s.db, s.template.ID, date), "the running materialization must not recreate the deleted occurrence")
			again, err := s.svc.MaterializeForTenant(s.ctx, date, date, timetable.MaterializationSourceManual)
			require.NoError(t, err)
			assert.Zero(t, again.InstancesCreated, "the deleted slot stays consumed")
		})
	}
}

// parkPoint holds the transaction that reaches it until release and hands
// the test its backend PID.
type parkPoint struct {
	t        *testing.T
	reached  chan int
	released chan struct{}
	once     sync.Once
}

func newParkPoint(t *testing.T) *parkPoint {
	t.Helper()
	p := &parkPoint{t: t, reached: make(chan int, 1), released: make(chan struct{})}
	t.Cleanup(p.release)
	return p
}

// park runs on the command's goroutine, so it reports failures as errors.
func (p *parkPoint) park(ctx context.Context) error {
	txValue, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return errors.New("park point reached outside a tenant transaction")
	}
	var pid int
	if err := txValue.(bun.Tx).NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid); err != nil {
		return err
	}
	select {
	case p.reached <- pid:
	default: // a later pass after release
	}
	select {
	case <-p.released:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *parkPoint) release() { p.once.Do(func() { close(p.released) }) }

// await returns the PID of the parked transaction; the command must reach
// the point before it finishes.
func (p *parkPoint) await(done <-chan error) int {
	p.t.Helper()
	select {
	case pid := <-p.reached:
		return pid
	case err := <-done:
		p.t.Fatalf("command finished before its park point: %v", err)
		return 0
	}
}

// parkedLifecycle is the lifecycle whose first read of the block parks at
// point: after the command's unlocked load, before anything it locks.
func parkedLifecycle(t *testing.T, s *lifecycleSetup, instanceID int64, point *parkPoint) fixtures.InstanceLifecycle {
	t.Helper()
	deps := lifecycleDependencies(s, nil)
	deps.InstanceRepo.Reads = fixtures.AfterFirstInstanceRead(deps.InstanceRepo.Reads, instanceID, point.park)
	return newLifecycle(t, deps)
}

// runInTenantTx runs fn in its own tenant transaction on another goroutine.
func runInTenantTx(t *testing.T, ctx context.Context, db *bun.DB, fn func(context.Context) error) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
			return fn(txCtx)
		})
	}()
	return done
}

// deleteInTenantTx runs DeleteCancelled in the tenant transaction its route
// opens: the delete holds its locks until commit.
func deleteInTenantTx(t *testing.T, ctx context.Context, db *bun.DB, svc interface {
	DeleteCancelled(context.Context, int64) error
}, instanceID int64) error {
	t.Helper()
	return <-runInTenantTx(t, ctx, db, func(txCtx context.Context) error {
		return svc.DeleteCancelled(txCtx, instanceID)
	})
}

// awaitBlockedOrDone returns once PostgreSQL reports a lock request blocked
// by the holder, or once the command finished without waiting. The ticker
// only paces observation; the go test timeout bounds a command that never
// queues (#5221/#5271).
func awaitBlockedOrDone(t *testing.T, ctx context.Context, db *bun.DB, holderPID int, done <-chan error) (bool, error) {
	t.Helper()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		var waiting bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks l
   WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))`, holderPID).Scan(ctx, &waiting))
		if waiting {
			return false, nil
		}
		select {
		case err := <-done:
			return true, err
		case <-poll.C:
		}
	}
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

// parkingStartConflicts parks a start after its locked re-read, before its
// first write.
type parkingStartConflicts struct {
	timetable.StartConflictQuery
	before func(context.Context) error
}

func (p parkingStartConflicts) DetectStartConflicts(ctx context.Context, subject timetable.StartConflictSubject) ([]timetable.InstanceConflictWarning, error) {
	if err := p.before(ctx); err != nil {
		return nil, err
	}
	return p.StartConflictQuery.DetectStartConflicts(ctx, subject)
}

// parkingExceptions parks a materialization run after it loaded the
// exceptions of its window, before it loads the existing blocks.
type parkingExceptions struct {
	timetable.ActivityExceptionQuery
	point *parkPoint
}

func (p parkingExceptions) ListActivityExceptions(ctx context.Context, filter timetable.ActivityExceptionFilter) ([]timetable.ActivityException, error) {
	rows, err := p.ActivityExceptionQuery.ListActivityExceptions(ctx, filter)
	if err != nil || filter.FromDate == nil {
		return rows, err
	}
	return rows, p.point.park(ctx)
}
