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
	t.Cleanup(func() { _ = db.Close() })

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
	t.Cleanup(func() {
		testpkg.CleanupActivityFixtures(t, db, 0, subject.ID, 0, tmpl.ID, room.ID)
		testpkg.CleanupStaffFixtures(t, db, admin.ID)
		testpkg.CleanupStaffFixtures(t, db, sub.ID)
	})

	syncer := scheduleSvc.NewShiftPlanSyncService(
		serviceFactory.StaffShifts,
		serviceFactory.Instance,
		serviceFactory.TimetableData,
		repoFactory.StaffShift,
		repoFactory.InstanceStaff,
		slog.Default(),
	)

	return &sickCascadeEnv{
		db:      db,
		repos:   repoFactory,
		factory: serviceFactory,
		syncer:  syncer,
		ctx:     testpkg.TenantContext(1),
		subject: &scheduleStaffRef{ID: subject.ID},
		admin:   &scheduleStaffRef{ID: admin.ID},
		sub:     &scheduleStaffRef{ID: sub.ID},
		roomID:  room.ID,
		tmplID:  tmpl.ID,
	}
}

func (e *sickCascadeEnv) inTx(t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	require.NoError(t, tenant.WithTenantTx(context.Background(), e.db, 1, func(txCtx context.Context, _ bun.Tx) error {
		return fn(txCtx)
	}))
}

func (e *sickCascadeEnv) clock(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	// time.Parse anchors at year 0, which Postgres rejects; WallClock re-anchors.
	return timezone.WallClock(parsed)
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
	shift.SetTenantID(1)
	require.NoError(t, e.repos.StaffShift.Create(e.ctx, shift))
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, e.db, "schedule.staff_shifts", shift.ID)
	})
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
	t.Cleanup(func() {
		testpkg.CleanupInstanceStaffFixtures(t, e.db, row.ID)
		testpkg.CleanupTableRecords(t, e.db, "schedule.activity_instances", instance.ID)
	})
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
		for _, ev := range filtered {
			testpkg.CleanupTableRecords(t, e.db, "audit.deviation_events", ev.ID)
		}
	})
	return filtered
}

func TestSickCascade_MarkSickForRange(t *testing.T) {
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
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, e.db, "active.staff_absences", absenceID)
	})

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

func TestSickCascade_HalfDayRules(t *testing.T) {
	e := buildSickCascadeEnv(t)
	today := timezone.TodayDate()
	tomorrow := today.AddDays(1)
	dayAfter := today.AddDays(2)

	shiftTomorrow := e.createShift(t, e.subject.ID, tomorrow, "08:00", "12:00", nil)
	shiftDayAfter := e.createShift(t, e.subject.ID, dayAfter, "08:00", "12:00", nil)

	// HalfDay absences never cascade (guarded in the absence service).
	e.inTx(t, func(ctx context.Context) error {
		resp, err := e.factory.StaffAbsence.CreateAbsenceFor(ctx, e.subject.ID, e.subject.ID, nil, activeSvc.CreateAbsenceRequest{
			AbsenceType: "sick",
			DateStart:   tomorrow.String(),
			DateEnd:     tomorrow.String(),
			HalfDay:     true,
		})
		if err != nil {
			return err
		}
		t.Cleanup(func() {
			testpkg.CleanupTableRecords(t, e.db, "active.staff_absences", resp.ID)
		})
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

func TestSickCascade_ClearSickForRange(t *testing.T) {
	e := buildSickCascadeEnv(t)
	today := timezone.TodayDate()
	tomorrow := today.AddDays(1)

	plain := e.createShift(t, e.subject.ID, tomorrow, "08:00", "12:00", nil)
	replaced := e.createShift(t, e.subject.ID, tomorrow, "13:00", "15:00", nil)
	_, clearableRow := e.createBlock(t, tomorrow, scheduleModels.InstanceStatusPlanned, "09:00", e.subject.ID, testpkg.InstanceStaffOpts{})
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

	// Admin work AFTER the cascade: a replacement covering one cancelled
	// shift, and a substitute on one marked block.
	cover := e.createShift(t, e.sub.ID, tomorrow, "13:00", "14:00", func(s *scheduleModels.StaffShift) {
		s.OriginShiftID = &replaced.ID
	})
	subRow := testpkg.CreateTestInstanceStaff(t, e.db, substitutedInstance.ID, e.sub.ID, testpkg.InstanceStaffOpts{IsSubstitute: true})
	t.Cleanup(func() {
		testpkg.CleanupInstanceStaffFixtures(t, e.db, subRow.ID)
	})

	e.inTx(t, func(ctx context.Context) error {
		return e.factory.StaffAbsence.DeleteAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, absenceID)
	})

	// Absence row is gone.
	_, err := e.repos.StaffAbsence.FindByID(e.ctx, absenceID)
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

func TestSickCascade_ReassignSickStamps(t *testing.T) {
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
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, e.db, "active.staff_absences", absenceID)
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
