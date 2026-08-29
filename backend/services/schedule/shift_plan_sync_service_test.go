// Hermetic integration tests for the #1843 sick cascade
// (shift_plan_sync_service.go) and its wiring through the staff absence
// service.
//
// Covers:
//   - MarkSickForRange: cancels + stamps only plain shifts (admin-cancelled
//     and replacement shifts stay untouched), marks only plannable
//     today/future instance rows (past, completed, already-absent skipped),
//     writes one sick_reported event per marked block, is idempotent.
//   - Half-day rules: HalfDay absences never cascade; boundary half days of a
//     range are skipped.
//   - ClearSickForRange: reactivates only cover-less stamped shifts, keeps
//     substituted blocks absent (stamp released), writes sick_cleared events.
//   - End-to-end: CreateAbsenceFor / DeleteAbsenceFor run the cascade inside
//     the tenant tx with subject/creator separation.
//
// All fixtures via testpkg.CreateTest* + Cleanup* — no hardcoded entity IDs.
package schedule_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type sickCascadeEnv struct {
	db      *bun.DB
	repos   *repositories.Factory
	factory *services.Factory
	syncer  activeSvc.ShiftPlanSyncer
	ctx     context.Context
	// tenantID is this test's own tenant (#2419); the env's raw-SQL and
	// WithTenantTx paths need the ID, not just the context.
	tenantID int64

	subject *scheduleStaffRef
	admin   *scheduleStaffRef
	sub     *scheduleStaffRef
	roomID  int64
	tmplID  int64
}

type scheduleStaffRef struct{ ID int64 }

func buildSickCascadeEnv(t *testing.T) *sickCascadeEnv {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err)

	suffix := time.Now().UnixNano()
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Sick-Room-%d", suffix))
	subject := testpkg.CreateTestStaff(t, db, "Sick", fmt.Sprintf("Subject-%d", suffix))
	admin := testpkg.CreateTestStaff(t, db, "Sick", fmt.Sprintf("Admin-%d", suffix))
	sub := testpkg.CreateTestStaff(t, db, "Sick", fmt.Sprintf("Sub-%d", suffix))
	tmpl := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Sick-Tmpl-%d", suffix))

	// Row-level cleanups are registered by each create helper; only the
	// parent fixtures are dropped here (LIFO: children first, then these).

	syncer := scheduleSvc.NewShiftPlanSyncService(
		serviceFactory.StaffShifts,
		serviceFactory.Instance,
		serviceFactory.TimetableData,
		repoFactory.StaffShift,
		repoFactory.InstanceStaff,
		nil,
		db,
		slog.Default(),
	)

	return &sickCascadeEnv{
		db:       db,
		repos:    repoFactory,
		factory:  serviceFactory,
		syncer:   syncer,
		ctx:      testpkg.Ctx(t),
		tenantID: testpkg.Tenant(t),
		subject:  &scheduleStaffRef{ID: subject.ID},
		admin:    &scheduleStaffRef{ID: admin.ID},
		sub:      &scheduleStaffRef{ID: sub.ID},
		roomID:   room.ID,
		tmplID:   tmpl.ID,
	}
}

func (e *sickCascadeEnv) inTx(t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	}))
}

func (e *sickCascadeEnv) clock(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	// time.Parse anchors at year 0, which Postgres rejects; WallClock re-anchors.
	return timezone.NormalizeWallClock(parsed)
}

// createShift inserts a staff shift row directly (fixture path, no service
// validation) and registers cleanup.
func (e *sickCascadeEnv) createShift(t *testing.T, staffID int64, date timezone.Date, start, end string, mutate func(*scheduleModels.StaffShift)) *scheduleModels.StaffShift {
	t.Helper()
	shift := &scheduleModels.StaffShift{
		StaffID:   staffID,
		Date:      date,
		StartTime: e.clock(t, start),
		EndTime:   e.clock(t, end),
		CreatedBy: staffID,
	}
	if mutate != nil {
		mutate(shift)
	}
	shift.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, e.repos.StaffShift.Create(e.ctx, shift))
	return shift
}

// createBlock inserts an instance + one staff row for the subject and
// registers cleanup. startHHMM must be unique per (template, date) — the
// template-instance unique index spans (activity_group_id, date, start_time).
func (e *sickCascadeEnv) createBlock(t *testing.T, date timezone.Date, status, startHHMM string, staffID int64, staffOpts testpkg.InstanceStaffOpts) (*scheduleModels.ActivityInstance, *scheduleModels.InstanceStaff) {
	t.Helper()
	instance := testpkg.CreateTestActivityInstance(t, e.db, date, e.roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &e.tmplID,
		Status:          status,
		StartHHMM:       startHHMM,
	})
	row := testpkg.CreateTestInstanceStaff(t, e.db, instance.ID, staffID, staffOpts)
	return instance, row
}

func (e *sickCascadeEnv) reloadShift(t *testing.T, id int64) *scheduleModels.StaffShift {
	t.Helper()
	shift, err := e.repos.StaffShift.FindByID(e.ctx, id)
	require.NoError(t, err)
	return shift
}

func (e *sickCascadeEnv) reloadRow(t *testing.T, id int64) *scheduleModels.InstanceStaff {
	t.Helper()
	row, err := e.repos.InstanceStaff.FindByID(e.ctx, id)
	require.NoError(t, err)
	return row
}

func (e *sickCascadeEnv) eventsByType(t *testing.T, from, to timezone.Date, eventType string) []*auditModels.DeviationEvent {
	t.Helper()
	events, err := e.repos.DeviationEvent.ListByRange(e.ctx, from, to, nil, nil)
	require.NoError(t, err)
	var filtered []*auditModels.DeviationEvent
	for _, ev := range events {
		if ev.EventType == eventType && ev.SubjectStaffID != nil && *ev.SubjectStaffID == e.subject.ID {
			filtered = append(filtered, ev)
		}
	}
	// Deviation events have no per-row cleanup helper; drop them by id.
	t.Cleanup(func() {
	})
	return filtered
}

func TestSickCascade_MarkSickForRange(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	today := timezone.TodayDate()
	tomorrow := today.AddDays(1)
	yesterday := today.AddDays(-1)

	// Shifts: one plain (cancel+stamp), one pre-cancelled by an admin (keep),
	// one replacement owned by the subject covering a colleague (skip).
	plain := e.createShift(t, e.subject.ID, tomorrow, "08:00", "12:00", nil)
	adminReason := "Fortbildung entfallen"
	preCancelled := e.createShift(t, e.subject.ID, tomorrow, "13:00", "15:00", func(s *scheduleModels.StaffShift) {
		s.Cancelled = true
		s.ChangeReason = &adminReason
	})
	colleagueGap := e.createShift(t, e.sub.ID, tomorrow, "15:00", "17:00", func(s *scheduleModels.StaffShift) {
		s.Cancelled = true
	})
	coverOfColleague := e.createShift(t, e.subject.ID, tomorrow, "15:00", "16:00", func(s *scheduleModels.StaffShift) {
		s.OriginShiftID = &colleagueGap.ID
	})

	// Blocks: plannable tomorrow (mark), completed tomorrow (skip), past
	// planned (skip), plannable but already absent (skip, no event).
	_, plannableRow := e.createBlock(t, tomorrow, scheduleModels.InstanceStatusPlanned, "09:00", e.subject.ID, testpkg.InstanceStaffOpts{})
	_, completedRow := e.createBlock(t, tomorrow, scheduleModels.InstanceStatusCompleted, "10:00", e.subject.ID, testpkg.InstanceStaffOpts{})
	_, pastRow := e.createBlock(t, yesterday, scheduleModels.InstanceStatusPlanned, "09:00", e.subject.ID, testpkg.InstanceStaffOpts{})
	_, absentRow := e.createBlock(t, tomorrow, scheduleModels.InstanceStatusPlanned, "11:00", e.subject.ID, testpkg.InstanceStaffOpts{IsAbsent: true})

	absenceID := int64(0)
	e.inTx(t, func(ctx context.Context) error {
		resp, err := e.factory.StaffAbsence.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, activeSvc.CreateAbsenceRequest{
			AbsenceType: "sick",
			DateStart:   yesterday.String(),
			DateEnd:     tomorrow.String(),
		})
		if err != nil {
			return err
		}
		absenceID = resp.ID
		return nil
	})
	require.NotZero(t, absenceID)

	// Subject/creator separation.
	absence, err := e.repos.StaffAbsence.FindByID(e.ctx, absenceID)
	require.NoError(t, err)
	assert.Equal(t, e.subject.ID, absence.StaffID)
	assert.Equal(t, e.admin.ID, absence.CreatedBy)

	// Plain shift: cancelled, neutral reason, provenance stamped. Past-day
	// shifts are included by design (they did not take place).
	reloaded := e.reloadShift(t, plain.ID)
	assert.True(t, reloaded.Cancelled)
	require.NotNil(t, reloaded.ChangeReason)
	assert.Equal(t, "Krankheit", *reloaded.ChangeReason)
	require.NotNil(t, reloaded.SickAbsenceID)
	assert.Equal(t, absenceID, *reloaded.SickAbsenceID)

	// Admin-cancelled shift: untouched (its reason and covers stay).
	reloaded = e.reloadShift(t, preCancelled.ID)
	assert.True(t, reloaded.Cancelled)
	require.NotNil(t, reloaded.ChangeReason)
	assert.Equal(t, adminReason, *reloaded.ChangeReason)
	assert.Nil(t, reloaded.SickAbsenceID)

	// Replacement shift owned by the subject: left in place, no stamp.
	reloaded = e.reloadShift(t, coverOfColleague.ID)
	assert.False(t, reloaded.Cancelled)
	assert.Nil(t, reloaded.SickAbsenceID)

	// Plannable block: marked absent with neutral reason + stamp.
	row := e.reloadRow(t, plannableRow.ID)
	assert.True(t, row.IsAbsent)
	require.NotNil(t, row.AbsenceReason)
	assert.Equal(t, "Krankmeldung", *row.AbsenceReason)
	require.NotNil(t, row.SickAbsenceID)
	assert.Equal(t, absenceID, *row.SickAbsenceID)

	// Completed / past / already-absent rows: untouched, never stamped.
	assert.False(t, e.reloadRow(t, completedRow.ID).IsAbsent)
	assert.False(t, e.reloadRow(t, pastRow.ID).IsAbsent)
	assert.Nil(t, e.reloadRow(t, completedRow.ID).SickAbsenceID)
	assert.Nil(t, e.reloadRow(t, pastRow.ID).SickAbsenceID)
	assert.Nil(t, e.reloadRow(t, absentRow.ID).SickAbsenceID)

	// Exactly one sick_reported event (the plannable block); the skipped rows
	// log nothing.
	events := e.eventsByType(t, yesterday, tomorrow, auditModels.DeviationEventSickReported)
	require.Len(t, events, 1)
	var newValue map[string]any
	require.NoError(t, json.Unmarshal(events[0].NewValue, &newValue))
	assert.Equal(t, "sick", newValue["cause"])
	assert.Equal(t, float64(absenceID), newValue["absence_id"])
	assert.Equal(t, true, newValue["is_absent"])

	// Idempotency: filing the same sick range again merges into the same
	// absence and re-runs the cascade as a no-op.
	e.inTx(t, func(ctx context.Context) error {
		resp, err := e.factory.StaffAbsence.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, activeSvc.CreateAbsenceRequest{
			AbsenceType: "sick",
			DateStart:   yesterday.String(),
			DateEnd:     tomorrow.String(),
		})
		if err != nil {
			return err
		}
		require.Equal(t, absenceID, resp.ID, "same-range sick report must merge into the primary absence")
		return nil
	})
	events = e.eventsByType(t, yesterday, tomorrow, auditModels.DeviationEventSickReported)
	assert.Len(t, events, 1, "idempotent re-run must not duplicate events")
}

func TestSickCascade_PastShiftsRemainHistoricalDuringMarkAndReconcile(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	yesterday := timezone.TodayDate().AddDays(-1)
	tomorrow := timezone.TodayDate().AddDays(1)
	past := e.createShift(t, e.subject.ID, yesterday, "08:00", "12:00", nil)

	e.inTx(t, func(ctx context.Context) error {
		return e.syncer.MarkSickForRange(ctx, activeSvc.SickCascadeInput{
			SubjectStaffID: e.subject.ID,
			ActorStaffID:   e.admin.ID,
			AbsenceID:      time.Now().UnixNano(),
			DateStart:      yesterday,
			DateEnd:        yesterday,
		})
	})
	stored := e.reloadShift(t, past.ID)
	assert.False(t, stored.Cancelled)
	assert.Nil(t, stored.SickAbsenceID)

	absenceID := e.createSickAbsence(t, tomorrow, tomorrow)
	reason := "Krankheit"
	stored.Cancelled = true
	stored.ChangeReason = &reason
	stored.SickAbsenceID = &absenceID
	_, err := e.repos.StaffShift.UpdateColumns(e.ctx, stored, "cancelled", "change_reason", "sick_absence_id")
	require.NoError(t, err)

	before := activeSvc.SickCascadeInput{
		SubjectStaffID: e.subject.ID,
		ActorStaffID:   e.admin.ID,
		AbsenceID:      absenceID,
		DateStart:      yesterday,
		DateEnd:        yesterday,
	}
	after := before
	after.DateStart = tomorrow
	after.DateEnd = tomorrow
	e.inTx(t, func(ctx context.Context) error {
		return e.syncer.ReconcileSickRange(ctx, before, after)
	})

	stored = e.reloadShift(t, past.ID)
	assert.True(t, stored.Cancelled, "reconcile must not reactivate a historical shift")
	require.NotNil(t, stored.ChangeReason)
	assert.Equal(t, reason, *stored.ChangeReason)
	assert.Nil(t, stored.SickAbsenceID, "reconcile should release obsolete provenance")
}

func TestSickCascade_ShiftOnlyChangesBroadcastTenantInvalidation(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	broadcaster := testpkg.NewRecordingBroadcaster()
	syncer := scheduleSvc.NewShiftPlanSyncService(
		e.factory.StaffShifts,
		e.factory.Instance,
		e.factory.TimetableData,
		e.repos.StaffShift,
		e.repos.InstanceStaff,
		broadcaster,
		e.db,
		slog.Default(),
	)
	dayOne := timezone.TodayDate().AddDays(1)
	dayTwo := dayOne.AddDays(1)
	e.createShift(t, e.subject.ID, dayOne, "08:00", "09:00", nil)
	e.createShift(t, e.subject.ID, dayTwo, "08:00", "09:00", nil)
	input := activeSvc.SickCascadeInput{
		SubjectStaffID: e.subject.ID,
		ActorStaffID:   e.admin.ID,
		AbsenceID:      time.Now().UnixNano(),
		DateStart:      dayOne,
		DateEnd:        dayOne,
	}

	e.inTx(t, func(ctx context.Context) error { return syncer.MarkSickForRange(ctx, input) })
	after := input
	after.DateStart = dayTwo
	after.DateEnd = dayTwo
	e.inTx(t, func(ctx context.Context) error { return syncer.ReconcileSickRange(ctx, input, after) })
	e.inTx(t, func(ctx context.Context) error { return syncer.ClearSickForRange(ctx, after) })

	calls := broadcaster.CallsByMethod("tenant")
	require.Len(t, calls, 3)
	expectedSources := []string{"sick_cascade_mark", "sick_cascade_reconcile", "sick_cascade_clear"}
	expectedTenantID := tenant.FromContext(e.ctx)
	for i, call := range calls {
		assert.Equal(t, expectedTenantID, call.TenantID)
		assert.Equal(t, realtime.EventStaffingDeviationChanged, call.Event.Type)
		require.NotNil(t, call.Event.Data.Source)
		assert.Equal(t, expectedSources[i], *call.Event.Data.Source)
	}
}

func TestSickCascade_ConcurrentOverlappingReportsSerializeBeforeOverlapRead(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	day := timezone.TodayDate().AddDays(1)

	lockHeld := make(chan struct{})
	releaseLock := make(chan struct{})
	lockerDone := make(chan error, 1)
	go func() {
		lockerDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			if err := e.repos.StaffAbsence.LockStaffAbsenceWrites(ctx, e.subject.ID); err != nil {
				return err
			}
			close(lockHeld)
			<-releaseLock
			_, err := e.factory.StaffAbsence.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, activeSvc.CreateAbsenceRequest{
				AbsenceType: "sick",
				DateStart:   day.String(),
				DateEnd:     day.String(),
			})
			return err
		})
	}()
	<-lockHeld

	creatorDone := make(chan error, 1)
	go func() {
		creatorDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			_, err := e.factory.StaffAbsence.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, activeSvc.CreateAbsenceRequest{
				AbsenceType: "sick",
				DateStart:   day.String(),
				DateEnd:     day.String(),
			})
			return err
		})
	}()

	select {
	case err := <-creatorDone:
		require.Failf(t, "concurrent create bypassed staff absence lock", "returned early: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	close(releaseLock)
	require.NoError(t, <-lockerDone)
	require.NoError(t, <-creatorDone)

	rows, err := e.repos.StaffAbsence.GetByStaffAndDateRange(e.ctx, e.subject.ID, day, day)
	require.NoError(t, err)
	require.Len(t, rows, 1, "serialized overlapping reports must merge into one absence")
}

func TestSickCascade_HalfDayRules(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	today := timezone.TodayDate()
	tomorrow := today.AddDays(1)
	dayAfter := today.AddDays(2)

	shiftTomorrow := e.createShift(t, e.subject.ID, tomorrow, "08:00", "12:00", nil)
	shiftDayAfter := e.createShift(t, e.subject.ID, dayAfter, "08:00", "12:00", nil)

	// HalfDay absences never cascade (guarded in the absence service).
	e.inTx(t, func(ctx context.Context) error {
		_, err := e.factory.StaffAbsence.CreateAbsenceFor(ctx, e.subject.ID, e.subject.ID, nil, activeSvc.CreateAbsenceRequest{
			AbsenceType: "sick",
			DateStart:   tomorrow.String(),
			DateEnd:     tomorrow.String(),
			HalfDay:     true,
		})
		if err != nil {
			return err
		}
		return nil
	})
	assert.False(t, e.reloadShift(t, shiftTomorrow.ID).Cancelled, "half-day sick report must not cancel shifts")

	// Boundary half days of a range are skipped by the syncer itself.
	e.inTx(t, func(ctx context.Context) error {
		return e.syncer.MarkSickForRange(ctx, activeSvc.SickCascadeInput{
			SubjectStaffID: e.subject.ID,
			DateStart:      tomorrow,
			DateEnd:        dayAfter,
			SkipStartDay:   true,
			AbsenceID:      424242, // direct syncer call: provenance target not persisted
			ActorStaffID:   e.subject.ID,
		})
	})
	assert.False(t, e.reloadShift(t, shiftTomorrow.ID).Cancelled, "skipped boundary half day must keep its shift")
	assert.True(t, e.reloadShift(t, shiftDayAfter.ID).Cancelled, "full day inside the range must cancel")
}

func TestSickCascade_ReconcileSickRangeAppliesOnlyDateDelta(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	dayOne := timezone.TodayDate().AddDays(1)
	dayTwo := dayOne.AddDays(1)
	dayThree := dayTwo.AddDays(1)

	shiftOne := e.createShift(t, e.subject.ID, dayOne, "08:00", "09:00", nil)
	shiftTwo := e.createShift(t, e.subject.ID, dayTwo, "08:00", "09:00", nil)
	shiftThree := e.createShift(t, e.subject.ID, dayThree, "08:00", "09:00", nil)
	_, rowOne := e.createBlock(t, dayOne, scheduleModels.InstanceStatusPlanned, "10:00", e.subject.ID, testpkg.InstanceStaffOpts{})
	_, rowTwo := e.createBlock(t, dayTwo, scheduleModels.InstanceStatusPlanned, "10:00", e.subject.ID, testpkg.InstanceStaffOpts{})
	_, rowThree := e.createBlock(t, dayThree, scheduleModels.InstanceStatusPlanned, "10:00", e.subject.ID, testpkg.InstanceStaffOpts{})

	var absenceID int64
	e.inTx(t, func(ctx context.Context) error {
		created, err := e.factory.StaffAbsence.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, activeSvc.CreateAbsenceRequest{
			AbsenceType: "sick",
			DateStart:   dayOne.String(),
			DateEnd:     dayTwo.String(),
		})
		if err == nil {
			absenceID = created.ID
		}
		return err
	})

	before := activeSvc.SickCascadeInput{
		AbsenceID:      absenceID,
		SubjectStaffID: e.subject.ID,
		ActorStaffID:   e.admin.ID,
		DateStart:      dayOne,
		DateEnd:        dayTwo,
	}

	unchangedShift := e.reloadShift(t, shiftTwo.ID)
	unchangedRow := e.reloadRow(t, rowTwo.ID)
	require.NotNil(t, unchangedShift.SickAbsenceID)
	require.NotNil(t, unchangedRow.SickAbsenceID)

	after := before
	after.DateStart = dayTwo
	after.DateEnd = dayThree
	e.inTx(t, func(ctx context.Context) error { return e.syncer.ReconcileSickRange(ctx, before, after) })

	assert.False(t, e.reloadShift(t, shiftOne.ID).Cancelled)
	assert.Nil(t, e.reloadShift(t, shiftOne.ID).SickAbsenceID)
	assert.True(t, e.reloadShift(t, shiftTwo.ID).Cancelled)
	assert.Equal(t, unchangedShift.SickAbsenceID, e.reloadShift(t, shiftTwo.ID).SickAbsenceID)
	assert.True(t, e.reloadShift(t, shiftThree.ID).Cancelled)

	assert.False(t, e.reloadRow(t, rowOne.ID).IsAbsent)
	assert.Nil(t, e.reloadRow(t, rowOne.ID).SickAbsenceID)
	assert.True(t, e.reloadRow(t, rowTwo.ID).IsAbsent)
	assert.Equal(t, unchangedRow.SickAbsenceID, e.reloadRow(t, rowTwo.ID).SickAbsenceID)
	assert.True(t, e.reloadRow(t, rowThree.ID).IsAbsent)

	e.eventsByType(t, dayOne, dayThree, auditModels.DeviationEventSickReported)
	e.eventsByType(t, dayOne, dayThree, auditModels.DeviationEventSickCleared)
}

func TestSickCascade_UpdateRangeRollsBackWhenRemovedShiftCannotReactivate(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	oldDay := timezone.TodayDate().AddDays(1)
	newDay := oldDay.AddDays(1)
	original := e.createShift(t, e.subject.ID, oldDay, "08:00", "10:00", nil)
	absenceID := e.createSickAbsence(t, oldDay, oldDay)

	// This active shift is valid while the original is sick-cancelled, but it
	// prevents the range edit from restoring the original on the removed day.
	e.createShift(t, e.subject.ID, oldDay, "09:00", "11:00", nil)
	newStart := newDay.String()
	newEnd := newDay.String()
	err := testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
		_, updateErr := e.factory.StaffAbsence.UpdateAbsence(ctx, e.subject.ID, nil, absenceID, activeSvc.UpdateAbsenceRequest{
			DateStart: &newStart,
			DateEnd:   &newEnd,
		})
		return updateErr
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan cascade reconciliation failed")

	absence, loadErr := e.repos.StaffAbsence.FindByID(e.ctx, absenceID)
	require.NoError(t, loadErr)
	assert.Equal(t, oldDay, absence.DateStart, "absence update must roll back with its cascade")
	assert.Equal(t, oldDay, absence.DateEnd)
	stored := e.reloadShift(t, original.ID)
	assert.True(t, stored.Cancelled)
	require.NotNil(t, stored.SickAbsenceID)
	assert.Equal(t, absenceID, *stored.SickAbsenceID)
}

func TestSickCascade_MarkWaitsForConcurrentShiftWrite(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	day := timezone.TodayDate().AddDays(1)
	absenceID := e.createSickAbsence(t, day, day)
	input := activeSvc.SickCascadeInput{
		AbsenceID:      absenceID,
		SubjectStaffID: e.subject.ID,
		ActorStaffID:   e.admin.ID,
		DateStart:      day,
		DateEnd:        day,
	}
	shift := &scheduleModels.StaffShift{
		StaffID: e.subject.ID, Date: day, StartTime: e.clock(t, "08:00"),
		EndTime: e.clock(t, "09:00"), CreatedBy: e.admin.ID,
	}
	shift.SetTenantID(testpkg.Tenant(t))

	created := make(chan struct{})
	release := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			if err := scheduleSvc.LockStaffShiftWrites(ctx, e.db, e.subject.ID); err != nil {
				return err
			}
			if err := e.repos.StaffShift.Create(ctx, shift); err != nil {
				return err
			}
			close(created)
			<-release
			return nil
		})
	}()
	<-created

	markDone := make(chan error, 1)
	go func() {
		markDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			return e.syncer.MarkSickForRange(ctx, input)
		})
	}()
	select {
	case err := <-markDone:
		require.Failf(t, "cascade did not wait for staff lock", "returned early: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-writerDone)
	require.NoError(t, <-markDone)

	stored := e.reloadShift(t, shift.ID)
	assert.True(t, stored.Cancelled)
	require.NotNil(t, stored.SickAbsenceID)
	assert.Equal(t, absenceID, *stored.SickAbsenceID)
}

func TestSickCascade_ClearWaitsForConcurrentReplacement(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	day := timezone.TodayDate().AddDays(1)
	origin := e.createShift(t, e.subject.ID, day, "08:00", "10:00", nil)
	absenceID := e.createSickAbsence(t, day, day)
	input := activeSvc.SickCascadeInput{
		AbsenceID:      absenceID,
		SubjectStaffID: e.subject.ID,
		ActorStaffID:   e.admin.ID,
		DateStart:      day,
		DateEnd:        day,
	}
	cover := &scheduleModels.StaffShift{
		StaffID: e.sub.ID, Date: day, StartTime: e.clock(t, "08:00"),
		EndTime: e.clock(t, "10:00"), CreatedBy: e.admin.ID, OriginShiftID: &origin.ID,
	}
	cover.SetTenantID(testpkg.Tenant(t))

	created := make(chan struct{})
	release := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			if err := scheduleSvc.LockStaffShiftWrites(ctx, e.db, e.subject.ID); err != nil {
				return err
			}
			if err := e.repos.StaffShift.Create(ctx, cover); err != nil {
				return err
			}
			close(created)
			<-release
			return nil
		})
	}()
	<-created

	clearDone := make(chan error, 1)
	go func() {
		clearDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			return e.syncer.ClearSickForRange(ctx, input)
		})
	}()
	select {
	case err := <-clearDone:
		require.Failf(t, "clear did not wait for staff lock", "returned early: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-writerDone)
	require.NoError(t, <-clearDone)

	stored := e.reloadShift(t, origin.ID)
	assert.True(t, stored.Cancelled, "replacement must keep its origin cancelled")
	assert.Nil(t, stored.SickAbsenceID, "deleted report must release ownership")
	_, err := e.repos.StaffShift.FindByID(e.ctx, cover.ID)
	require.NoError(t, err, "concurrent replacement must survive the clear")
}

func TestSickCascade_ClearLocksCommittedReplacementStaffBeforeReversal(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	day := timezone.TodayDate().AddDays(1)
	origin := e.createShift(t, e.subject.ID, day, "08:00", "10:00", nil)
	absenceID := e.createSickAbsence(t, day, day)
	cover := e.createShift(t, e.sub.ID, day, "08:00", "10:00", func(shift *scheduleModels.StaffShift) {
		shift.OriginShiftID = &origin.ID
	})
	input := activeSvc.SickCascadeInput{
		AbsenceID:      absenceID,
		SubjectStaffID: e.subject.ID,
		ActorStaffID:   e.admin.ID,
		DateStart:      day,
		DateEnd:        day,
	}

	locked := make(chan struct{})
	release := make(chan struct{})
	lockerDone := make(chan error, 1)
	go func() {
		lockerDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			if err := scheduleSvc.LockStaffShiftWrites(ctx, e.db, cover.StaffID); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked

	clearDone := make(chan error, 1)
	go func() {
		clearDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			return e.syncer.ClearSickForRange(ctx, input)
		})
	}()
	select {
	case err := <-clearDone:
		require.Failf(t, "clear did not lock replacement staff", "returned early: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-lockerDone)
	require.NoError(t, <-clearDone)

	stored := e.reloadShift(t, origin.ID)
	assert.True(t, stored.Cancelled, "replacement must keep its origin cancelled")
	assert.Nil(t, stored.SickAbsenceID, "reversal must release the sick-report stamp")
	_, err := e.repos.StaffShift.FindByID(e.ctx, cover.ID)
	require.NoError(t, err, "replacement must survive the reversal")
}

func (e *sickCascadeEnv) createSickAbsence(t *testing.T, start, end timezone.Date) int64 {
	t.Helper()
	var absenceID int64
	e.inTx(t, func(ctx context.Context) error {
		created, err := e.factory.StaffAbsence.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, activeSvc.CreateAbsenceRequest{
			AbsenceType: "sick",
			DateStart:   start.String(),
			DateEnd:     end.String(),
		})
		if err == nil {
			absenceID = created.ID
		}
		return err
	})
	return absenceID
}

func TestSickCascade_ClearSickForRange(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	today := timezone.TodayDate()
	tomorrow := today.AddDays(1)

	plain := e.createShift(t, e.subject.ID, tomorrow, "08:00", "12:00", nil)
	replaced := e.createShift(t, e.subject.ID, tomorrow, "13:00", "15:00", nil)
	clearableInstance, clearableRow := e.createBlock(t, tomorrow, scheduleModels.InstanceStatusPlanned, "09:00", e.subject.ID, testpkg.InstanceStaffOpts{})
	substitutedInstance, substitutedRow := e.createBlock(t, tomorrow, scheduleModels.InstanceStatusPlanned, "10:00", e.subject.ID, testpkg.InstanceStaffOpts{})

	var absenceID int64
	e.inTx(t, func(ctx context.Context) error {
		resp, err := e.factory.StaffAbsence.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, activeSvc.CreateAbsenceRequest{
			AbsenceType: "sick",
			DateStart:   tomorrow.String(),
			DateEnd:     tomorrow.String(),
		})
		if err != nil {
			return err
		}
		absenceID = resp.ID
		return nil
	})
	ackNote := "bewusst offen"
	clearableInstance.UnderstaffedAck = true
	clearableInstance.UnderstaffedNote = &ackNote
	_, err := e.repos.ActivityInstance.UpdateColumns(e.ctx, clearableInstance, "understaffed_ack", "understaffed_note")
	require.NoError(t, err)

	// Admin work AFTER the cascade: a replacement covering one cancelled
	// shift, and a substitute on one marked block.
	cover := e.createShift(t, e.sub.ID, tomorrow, "13:00", "14:00", func(s *scheduleModels.StaffShift) {
		s.OriginShiftID = &replaced.ID
	})
	subRow := testpkg.CreateTestInstanceStaff(t, e.db, substitutedInstance.ID, e.sub.ID, testpkg.InstanceStaffOpts{IsSubstitute: true})

	e.inTx(t, func(ctx context.Context) error {
		return e.factory.StaffAbsence.DeleteAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, absenceID)
	})

	// Absence row is gone.
	_, err = e.repos.StaffAbsence.FindByID(e.ctx, absenceID)
	assert.Error(t, err, "deleted absence must not be found")

	// Cover-less shift: reactivated, reason and stamp cleared.
	reloaded := e.reloadShift(t, plain.ID)
	assert.False(t, reloaded.Cancelled)
	assert.Nil(t, reloaded.ChangeReason)
	assert.Nil(t, reloaded.SickAbsenceID)

	// Replaced shift: stays cancelled (reactivation would delete the admin's
	// cover), stamp released, cover survives.
	reloaded = e.reloadShift(t, replaced.ID)
	assert.True(t, reloaded.Cancelled)
	assert.Nil(t, reloaded.SickAbsenceID)
	coverReloaded := e.reloadShift(t, cover.ID)
	require.NotNil(t, coverReloaded.OriginShiftID)

	// Clearable block: presence restored, stamp cleared, sick_cleared logged.
	row := e.reloadRow(t, clearableRow.ID)
	assert.False(t, row.IsAbsent)
	assert.Nil(t, row.AbsenceReason)
	assert.Nil(t, row.SickAbsenceID)
	clearableInstance, err = e.repos.ActivityInstance.FindByID(e.ctx, clearableInstance.ID)
	require.NoError(t, err)
	assert.False(t, clearableInstance.UnderstaffedAck)
	assert.Nil(t, clearableInstance.UnderstaffedNote)
	allEvents, err := e.repos.DeviationEvent.ListByRange(e.ctx, tomorrow, tomorrow, nil, nil)
	require.NoError(t, err)
	var unackEvents []*auditModels.DeviationEvent
	for _, event := range allEvents {
		if event.EventType == auditModels.DeviationEventUnderstaffedUnack &&
			event.InstanceID != nil && *event.InstanceID == clearableInstance.ID {
			unackEvents = append(unackEvents, event)
		}
	}
	require.Len(t, unackEvents, 1)

	// Substituted block: stays absent (never silently overstaff), stamp
	// released, substitute row untouched.
	row = e.reloadRow(t, substitutedRow.ID)
	assert.True(t, row.IsAbsent)
	assert.Nil(t, row.SickAbsenceID)
	assert.False(t, e.reloadRow(t, subRow.ID).IsAbsent)

	cleared := e.eventsByType(t, tomorrow, tomorrow, auditModels.DeviationEventSickCleared)
	require.Len(t, cleared, 1, "exactly the restored block logs sick_cleared")
	reported := e.eventsByType(t, tomorrow, tomorrow, auditModels.DeviationEventSickReported)
	assert.Len(t, reported, 2, "both blocks were marked by the create cascade")
}

func TestSickCascade_DeleteDoesNotReactivateManuallyRetainedCancellation(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	day := timezone.TodayDate().AddDays(1)
	shift := e.createShift(t, e.subject.ID, day, "08:00", "12:00", nil)
	absenceID := e.createSickAbsence(t, day, day)

	stamped := e.reloadShift(t, shift.ID)
	require.NotNil(t, stamped.SickAbsenceID)
	assert.Equal(t, absenceID, *stamped.SickAbsenceID)

	manualReason := "Teamtag"
	e.inTx(t, func(ctx context.Context) error {
		_, err := e.factory.StaffShifts.ApplyCancellation(ctx, scheduleSvc.CancelShiftInput{
			ShiftID:      shift.ID,
			Cancelled:    true,
			ChangeReason: &manualReason,
			ActorStaffID: e.admin.ID,
		})
		return err
	})

	manuallyRetained := e.reloadShift(t, shift.ID)
	assert.True(t, manuallyRetained.Cancelled)
	assert.Nil(t, manuallyRetained.SickAbsenceID)

	e.inTx(t, func(ctx context.Context) error {
		return e.factory.StaffAbsence.DeleteAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, absenceID)
	})

	stored := e.reloadShift(t, shift.ID)
	assert.True(t, stored.Cancelled)
	assert.Nil(t, stored.SickAbsenceID)
	require.NotNil(t, stored.ChangeReason)
	assert.Equal(t, manualReason, *stored.ChangeReason)
}

func TestSickCascade_ReassignSickStamps(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	tomorrow := timezone.TodayDate().AddDays(1)

	shift := e.createShift(t, e.subject.ID, tomorrow, "08:00", "12:00", nil)
	_, row := e.createBlock(t, tomorrow, scheduleModels.InstanceStatusPlanned, "09:00", e.subject.ID, testpkg.InstanceStaffOpts{})

	var absenceID int64
	e.inTx(t, func(ctx context.Context) error {
		resp, err := e.factory.StaffAbsence.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, activeSvc.CreateAbsenceRequest{
			AbsenceType: "sick",
			DateStart:   tomorrow.String(),
			DateEnd:     tomorrow.String(),
		})
		if err != nil {
			return err
		}
		absenceID = resp.ID
		return nil
	})
	e.eventsByType(t, tomorrow, tomorrow, auditModels.DeviationEventSickReported) // registers event cleanup

	newOwner := absenceID + 1000
	e.inTx(t, func(ctx context.Context) error {
		return e.syncer.ReassignSickStamps(ctx, absenceID, newOwner)
	})

	reloadedShift := e.reloadShift(t, shift.ID)
	require.NotNil(t, reloadedShift.SickAbsenceID)
	assert.Equal(t, newOwner, *reloadedShift.SickAbsenceID)
	reloadedRow := e.reloadRow(t, row.ID)
	require.NotNil(t, reloadedRow.SickAbsenceID)
	assert.Equal(t, newOwner, *reloadedRow.SickAbsenceID)
}
