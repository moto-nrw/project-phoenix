// Concurrent integration tests for the lock protocol of a sick-report write
// against the plan writers (#6232, races carried over from #6300 and #6354).
//
// Invariants (each test names the one it pins):
//   - R1: a sick-report write takes, before any Workforce staff lock, the
//     staff-shift gate of its subject and the day locks of every day it may
//     stamp or release (past days included), ascending. A plan writer that
//     reads sick reports holds the days it writes (materialization, series
//     roster change) or the subject's staff-shift gate (series
//     materialization), so one of the two waits for the other's commit.
//   - R2: no block row is planned without the stamp of a committed report
//     that covers its day, and no row keeps the stamp of a deleted or merged
//     report.
//   - R3: the canonical order staff-shift gate → recurrence → days ascending
//     → rows holds on every path, so a day-lock writer, a series write and a
//     sick report never close a cycle (no 40P01).
//   - R4: the Workforce absence gate comes first, so two writes of one
//     subject read each other's committed ranges and never need a day after
//     a later one (no out-of-order day lock).
//
// Every test runs the real Workforce absence writes with their cascade and
// the real Timetable or Workforce plan writers; only the interleaving is
// synchronized.
package compose_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Mondays before the suite's fixed clock (2026-08-24).
var (
	racePastWeekOne = calendar.NewDate(2026, time.August, 10)
	racePastWeekTwo = calendar.NewDate(2026, time.August, 17)
)

// heldWrite is a transaction that ran its write and stays open until release
// closes.
type heldWrite struct {
	release chan struct{}
	done    chan error
}

// commit releases the held transaction and returns its outcome.
func (h heldWrite) commit() error {
	close(h.release)
	return <-h.done
}

// holdWrite runs write in its own tenant transaction and keeps the
// transaction, with its locks and uncommitted rows, open until the caller
// commits it. It fails the test when write fails.
func (e *sickCascadeEnv) holdWrite(t *testing.T, write func(ctx context.Context) error) heldWrite {
	t.Helper()
	written := make(chan struct{})
	held := heldWrite{release: make(chan struct{}), done: make(chan error, 1)}
	go func() {
		held.done <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			if err := write(ctx); err != nil {
				return err
			}
			close(written)
			<-held.release
			return nil
		})
	}()
	select {
	case <-written:
	case err := <-held.done:
		require.FailNow(t, "the held write failed before it held its locks", "%v", err)
	}
	return held
}

// contender is a write running in its own tenant transaction whose backend
// the test can watch.
type contender struct {
	pid  chan int64
	done chan error
}

// startContender runs write in its own tenant transaction; the transaction
// reports its backend first.
func (e *sickCascadeEnv) startContender(t *testing.T, write func(ctx context.Context) error) contender {
	t.Helper()
	c := contender{pid: make(chan int64, 1), done: make(chan error, 1)}
	go func() {
		c.done <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, tx bun.Tx) error {
			var pid int64
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &pid); err != nil {
				return err
			}
			c.pid <- pid
			return write(ctx)
		})
	}()
	return c
}

// awaitLockWait reports whether the contender waits for a lock (false) or
// finished first (true, with its outcome). It polls the contender's own
// backend on a private pool, at most five seconds.
func (e *sickCascadeEnv) awaitLockWait(t *testing.T, c contender) (bool, error) {
	t.Helper()
	var pid int64
	select {
	case pid = <-c.pid:
	case err := <-c.done:
		return true, err
	}
	poller := testpkg.SetupClosableTestDB(t)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-c.done:
			return true, err
		default:
		}
		var waitType sql.NullString
		require.NoError(t, poller.NewRaw(`SELECT wait_event_type FROM pg_stat_activity WHERE pid = ?`, pid).Scan(context.Background(), &waitType))
		if waitType.String == "Lock" {
			return false, nil
		}
	}
	require.FailNow(t, "the contender neither waited for a lock nor finished")
	return false, nil
}

// requireWaitsForHeld requires the contender to wait for the held write,
// then commits the held write and requires both to succeed. A contender that
// does not wait is reported, and the outcome assertions of the caller still
// run, so a missing lock shows its wrong result too.
func (e *sickCascadeEnv) requireWaitsForHeld(t *testing.T, held heldWrite, c contender, what string) {
	t.Helper()
	returned, early := e.awaitLockWait(t, c)
	require.NoError(t, held.commit())
	if returned {
		assert.Failf(t, what+" did not wait for the held write", "returned early: %v", early)
		require.NoError(t, early)
		return
	}
	require.NoError(t, <-c.done)
}

func (e *sickCascadeEnv) createSickReport(ctx context.Context, staffID int64, start, end calendar.Date) (int64, error) {
	created, err := e.absences.SickReports.CreateAbsenceFor(ctx, staffID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
		AbsenceType: "sick", DateStart: start.String(), DateEnd: end.String(),
	})
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (e *sickCascadeEnv) materializeIn(ctx context.Context, day calendar.Date) error {
	_, err := e.timetable.Materialization.MaterializeForTenant(ctx, day, day, timetable.MaterializationSourceManual)
	return err
}

// R1 + R2: a future sick report is open while its day is materialized. The
// run waits for the report's commit and plans the person absent.
func TestSickReport_MaterializationWaitsForConcurrentReport(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	var absenceID int64
	held := e.holdWrite(t, func(ctx context.Context) (err error) {
		absenceID, err = e.createSickReport(ctx, e.subject.ID, stampWeekOne, stampWeekOne)
		return err
	})
	run := e.startContender(t, func(ctx context.Context) error { return e.materializeIn(ctx, stampWeekOne) })
	e.requireWaitsForHeld(t, held, run, "materialization")

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), absenceID, "block materialized beside the open report")
}

// R1 + R2: a future sick report is deleted while its day is materialized. The
// run waits and plans the person present, without the deleted stamp.
func TestSickReport_MaterializationWaitsForConcurrentDeletion(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekOne)
	held := e.holdWrite(t, func(ctx context.Context) error {
		return e.absences.SickReports.DeleteAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, absenceID)
	})
	run := e.startContender(t, func(ctx context.Context) error { return e.materializeIn(ctx, stampWeekOne) })
	e.requireWaitsForHeld(t, held, run, "materialization")

	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), "block materialized beside the deletion")
}

// R1 + R2: a past day is materialized, stamping report S, while S is deleted.
// The deletion waits for the run and releases the stamp it wrote.
func TestSickReport_PastMaterializationAgainstDeletion(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	absenceID := e.createSickAbsence(t, racePastWeekTwo, racePastWeekTwo)
	held := e.holdWrite(t, func(ctx context.Context) error { return e.materializeIn(ctx, racePastWeekTwo) })
	deletion := e.startContender(t, func(ctx context.Context) error {
		return e.absences.SickReports.DeleteAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, absenceID)
	})
	e.requireWaitsForHeld(t, held, deletion, "deletion")

	row := e.templateRow(t, templateID, e.subject.ID, racePastWeekTwo)
	assert.Nil(t, row.SickAbsenceID, "no row keeps the stamp of the deleted report")
}

// R1 + R2: a past day is materialized, stamping report S, while a bridging
// report merges S into P. The merge waits for the run and moves its stamp to
// the survivor.
func TestSickReport_PastMaterializationAgainstMerge(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	survivorID := e.createSickAbsence(t, racePastWeekOne, racePastWeekOne)
	mergedID := e.createSickAbsence(t, racePastWeekTwo, racePastWeekTwo)
	held := e.holdWrite(t, func(ctx context.Context) error { return e.materializeIn(ctx, racePastWeekTwo) })
	merge := e.startContender(t, func(ctx context.Context) error {
		_, err := e.createSickReport(ctx, e.subject.ID, racePastWeekOne, racePastWeekTwo)
		return err
	})
	e.requireWaitsForHeld(t, held, merge, "merge")

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, racePastWeekTwo), survivorID, "materialized row of the merged report")
	_, err := e.shifts.FindStaffAbsence(e.ctx, mergedID)
	require.Error(t, err, "the merged report is gone")
}

// R1 + R2: a report for a past day is open while that day is materialized.
// The run waits and stamps its row.
func TestSickReport_PastReportAgainstMaterialization(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	var absenceID int64
	held := e.holdWrite(t, func(ctx context.Context) (err error) {
		absenceID, err = e.createSickReport(ctx, e.subject.ID, racePastWeekTwo, racePastWeekTwo)
		return err
	})
	run := e.startContender(t, func(ctx context.Context) error { return e.materializeIn(ctx, racePastWeekTwo) })
	e.requireWaitsForHeld(t, held, run, "materialization")

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, racePastWeekTwo), absenceID, "past block materialized beside the report")
}

// R1 + R2: a series roster change adds a person to a planned occurrence while
// that person's sick report for the day is open. The change waits and adds
// the person absent.
func TestSickReport_SeriesRosterChangeWaitsForConcurrentReport(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekOne)
	_, err := e.db.NewRaw(`INSERT INTO activities.supervisors (tenant_id, staff_id, group_id, is_primary, valid_from) VALUES (?, ?, ?, FALSE, ?)`,
		e.tenantID, e.sub.ID, templateID, calendar.NewDate(2026, time.August, 1)).Exec(e.ctx)
	require.NoError(t, err)
	var absenceID int64
	held := e.holdWrite(t, func(ctx context.Context) (err error) {
		absenceID, err = e.createSickReport(ctx, e.sub.ID, stampWeekOne, stampWeekOne)
		return err
	})
	align := e.startContender(t, func(ctx context.Context) error {
		return e.timetable.TimetableData.Templates.AlignPlannedInstanceStaff(ctx, templateID, []int64{e.sub.ID}, stampWeekOne)
	})
	e.requireWaitsForHeld(t, held, align, "series roster change")

	assertSickStamp(t, e.templateRow(t, templateID, e.sub.ID, stampWeekOne), absenceID, "added beside the open report")
}

// requireNoDeadlock waits for both transactions and requires that neither
// ended with 40P01 or any other error.
func requireNoDeadlock(t *testing.T, outcomes map[string]chan error) {
	t.Helper()
	for name, done := range outcomes {
		select {
		case err := <-done:
			require.NoError(t, err, "%s must not be a deadlock victim (40P01) or fail", name)
		case <-time.After(30 * time.Second):
			require.FailNow(t, name+" never finished")
		}
	}
}

// lockKey takes one transaction-scoped advisory lock the way the owners'
// adapters hash their keys.
func lockKey(ctx context.Context, tx bun.Tx, key string) error {
	_, err := tx.NewRaw("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Exec(ctx)
	return err
}

// R3: a writer holds a day lock and then inserts an assignment of the
// subject (the foreign key takes a key share on the staff row), as the
// materializer and every day-lock writer do, while a sick report for that day
// is filed. Before #6232 the report locked the staff row first and then
// waited for the day: 40P01. It now waits for the day before any staff row
// lock, and then marks the row the writer added.
func TestSickReport_DayLockWriterAndReportDoNotDeadlock(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	block, _ := e.createBlock(t, stampWeekOne, timetable.InstanceStatusPlanned, "10:00", e.admin.ID, testpkg.InstanceStaffOpts{})
	holding, proceed := make(chan struct{}), make(chan struct{})
	writer := make(chan error, 1)
	go func() {
		writer <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, tx bun.Tx) error {
			if err := lockKey(ctx, tx, timetable.SubstituteDayLockKey(e.tenantID, stampWeekOne)); err != nil {
				return err
			}
			close(holding)
			<-proceed
			_, err := tx.NewRaw(`INSERT INTO schedule.instance_staff (tenant_id, instance_id, staff_id) VALUES (?, ?, ?)`,
				e.tenantID, block.ID, e.subject.ID).Exec(ctx)
			return err
		})
	}()
	<-holding
	report := e.startContender(t, func(ctx context.Context) error {
		_, err := e.createSickReport(ctx, e.subject.ID, stampWeekOne, stampWeekOne)
		return err
	})
	returned, early := e.awaitLockWait(t, report)
	close(proceed)
	if returned {
		require.NoError(t, <-writer)
		require.FailNow(t, "the sick report did not wait for the held day", "returned early: %v", early)
	}
	requireNoDeadlock(t, map[string]chan error{"day-lock writer": writer, "sick report": report.done})

	row := e.instanceRow(t, block.ID, e.subject.ID)
	assert.True(t, row.IsAbsent, "the report marks the row the writer added")
	assert.NotNil(t, row.SickAbsenceID, "with its stamp")
}

// R3 (#6354): a shift series write holds the subject's staff-shift gate and
// then creates the series (its rows take the staff row) while a sick report
// for the subject is filed. Before #6232 the report locked the staff row
// and then waited for the gate in its cascade: 40P01. It now takes the gate
// first, waits, and cancels the new shift on the sick day.
func TestSickReport_SeriesWriteAndReportDoNotDeadlock(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	periodID := e.createSeriesPeriod(t)
	holding, proceed := make(chan struct{}), make(chan struct{})
	var seriesID int64
	series := make(chan error, 1)
	go func() {
		series <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, tx bun.Tx) error {
			// The key Workforce's staff-shift gate hashes (LockStaffShiftWrites).
			if err := lockKey(ctx, tx, fmt.Sprintf("staff-shift:%d:%d", e.tenantID, e.subject.ID)); err != nil {
				return err
			}
			close(holding)
			<-proceed
			result, err := e.Planning.CreateSeries(ctx, workforce.CreateStaffShiftSeries{StaffShiftSeriesInput: workforce.StaffShiftSeriesInput{
				StaffID: e.subject.ID, Weekdays: []int{1}, StartTime: clock("08:00"), EndTime: clock("12:00"),
				CalendarPeriodID: periodID, ValidFrom: stampWeekOne.String(), ValidUntil: stampWeekThree.AddDays(1).String(),
				ActorStaffID: e.admin.ID,
			}})
			seriesID = result.SeriesID
			return err
		})
	}()
	<-holding
	var absenceID int64
	report := e.startContender(t, func(ctx context.Context) (err error) {
		absenceID, err = e.createSickReport(ctx, e.subject.ID, stampWeekTwo, stampWeekTwo)
		return err
	})
	returned, early := e.awaitLockWait(t, report)
	close(proceed)
	if returned {
		require.NoError(t, <-series)
		require.FailNow(t, "the sick report did not wait for the held gate", "returned early: %v", early)
	}
	requireNoDeadlock(t, map[string]chan error{"series write": series, "sick report": report.done})

	assertSickCancelled(t, e.seriesShift(t, seriesID, stampWeekTwo), absenceID, "series shift on the sick day")
	assertPlanned(t, e.seriesShift(t, seriesID, stampWeekOne), "series shift on a healthy day")
}

// R4: a report is open while a second report of the same subject extends its
// range. The second write reads the first only after its commit, merges it
// and locks every day the merge stamps in one ascending pass: it waits and
// then succeeds, with the merged survivor stamping both blocks.
func TestSickReport_ConcurrentRangeExtensionMergesWithoutLockOrderFailure(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekThree)
	var firstID int64
	held := e.holdWrite(t, func(ctx context.Context) (err error) {
		firstID, err = e.createSickReport(ctx, e.subject.ID, stampWeekOne, stampWeekTwo)
		return err
	})
	var extendedID int64
	extension := e.startContender(t, func(ctx context.Context) (err error) {
		extendedID, err = e.createSickReport(ctx, e.subject.ID, stampWeekTwo, stampWeekThree)
		return err
	})
	e.requireWaitsForHeld(t, held, extension, "range extension")

	assert.Equal(t, firstID, extendedID, "the extension merged into the first report")
	for _, day := range []calendar.Date{stampWeekOne, stampWeekTwo, stampWeekThree} {
		assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, day), firstID, "block of the merged range "+day.String())
	}
}

// R4 for an edit that moves one bound only: Workforce keeps the stored other
// bound, and the write locks the days of the resulting range before the
// cascade marks the new first day.
func TestSickReport_EditMovingOnlyTheStartLocksItsNewDays(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekTwo)
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	start := stampWeekOne.String()
	e.inTx(t, func(ctx context.Context) error {
		_, err := e.absences.SickReports.UpdateAbsence(ctx, e.subject.ID, nil, absenceID, workforce.UpdateAbsenceRequest{DateStart: &start})
		return err
	})

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), absenceID, "the new first day")
	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekTwo), absenceID, "the kept day")
}

func TestSickReport_OversizedRangeDoesNotWaitForPlanLocks(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	e := buildSickCascadeEnv(t)
	absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekOne)
	held := e.holdWrite(t, func(ctx context.Context) error {
		return e.timetable.TimetableData.RecurrenceLock.LockRecurrenceWrites(ctx)
	})
	end := "9999-12-31"
	c := e.startContender(t, func(ctx context.Context) error {
		_, err := e.absences.SickReports.UpdateAbsence(ctx, e.subject.ID, nil, absenceID, workforce.UpdateAbsenceRequest{DateEnd: &end})
		return err
	})
	returned, early := e.awaitLockWait(t, c)
	require.NoError(t, held.commit())
	if !returned {
		early = <-c.done
	}
	require.True(t, returned, "an invalid partial edit must reach validation before the recurrence gate")
	require.ErrorIs(t, early, workforce.ErrAbsenceInvalid)

	_, err := e.absences.SickReports.CreateAbsenceFor(e.ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
		AbsenceType: workforce.AbsenceTypeSick, DateStart: stampWeekOne.String(), DateEnd: end,
	})
	require.ErrorIs(t, err, workforce.ErrAbsenceInvalid)
}

func TestSickReport_RevalidatesCoverOwnersAfterWaitingForStaffGates(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	for _, change := range []string{"create", "move"} {
		for _, action := range []string{"delete", "shorten"} {
			t.Run(change+"_cover_then_"+action, func(t *testing.T) {
				testpkg.OwnTenant(t)
				e := buildSickCascadeEnv(t)
				// The new cover owner sorts before every gate the initial scope
				// knows. Adding the missing gate in place cannot preserve the order.
				e.subject, e.sub = e.sub, e.subject
				origin := e.mustPlanShift(t, e.subject.ID, stampWeekTwo, "08:00", "10:00")
				absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekTwo)
				var cover workforce.StaffShift
				if change == "move" {
					var err error
					cover, err = e.planShift(t, e.admin.ID, stampWeekTwo, "08:00", "10:00", func(input *workforce.StaffShiftInput) {
						input.OriginShiftID = &origin.ID
					})
					require.NoError(t, err)
				}
				var coverID int64
				held := e.holdWrite(t, func(ctx context.Context) error {
					var planned workforce.PlannedShift
					var err error
					if change == "move" {
						planned, err = e.Planning.MoveShift(ctx, workforce.MoveStaffShift{
							ShiftID: cover.ID, SourceStaffID: cover.StaffID, TargetStaffID: e.sub.ID,
							Date: stampWeekTwo.String(), StartTime: clock("08:00"), EndTime: clock("10:00"), ActorStaffID: e.admin.ID,
						})
					} else {
						planned, err = e.Planning.CreateShift(ctx, workforce.CreateStaffShift{StaffShiftInput: workforce.StaffShiftInput{
							StaffID: e.sub.ID, Date: stampWeekTwo.String(), StartTime: clock("08:00"), EndTime: clock("10:00"),
							OriginShiftID: &origin.ID, ActorStaffID: e.admin.ID,
						}})
					}
					coverID = planned.ID
					return err
				})
				c := e.startContender(t, func(ctx context.Context) error {
					if action == "delete" {
						return e.absences.SickReports.DeleteAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, absenceID)
					}
					end := stampWeekOne.String()
					_, err := e.absences.SickReports.UpdateAbsence(ctx, e.subject.ID, nil, absenceID, workforce.UpdateAbsenceRequest{DateEnd: &end})
					return err
				})
				e.requireWaitsForHeld(t, held, c, "sick report write")
				kept := e.reloadShift(t, origin.ID)
				require.Nil(t, kept.SickAbsenceID, "the removed sick day must release its stamp")
				require.True(t, kept.Cancelled, "the committed cover keeps the origin cancelled")
				require.Equal(t, e.sub.ID, e.reloadShift(t, coverID).StaffID)
			})
		}
	}
}
