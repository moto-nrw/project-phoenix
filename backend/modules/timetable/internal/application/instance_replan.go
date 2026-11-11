package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ReplanWeek deletes the planned, template-backed blocks of [from, to] for
// the current tenant, re-materializes the window and reapplies the
// Vertretungsplan overrides. Everything else survives. A non-nil
// activityGroupID narrows the delete to one template; the insert-only
// materialization still covers the whole window.
func (s *InstanceLifecycleService) ReplanWeek(ctx context.Context, from, to calendar.Date, activityGroupID, actorAccountID *int64) (*timetable.ReplanWeekResult, error) {
	if to.Before(from) {
		return nil, &ports.ScheduleError{Op: "replan week: validate window", Err: errors.New("to_date must not be before from_date")}
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		// Without a tenant the delete would silently match nothing; fail
		// fast instead of no-oping the admin action.
		return nil, &ports.ScheduleError{Op: "replan week", Err: errors.New("no tenant in context")}
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		var result *timetable.ReplanWeekResult
		err := inTenant(ctx, s.deps.Database, tenantID, func(txCtx context.Context) error {
			var err error
			result, err = s.ReplanWeek(txCtx, from, to, activityGroupID, actorAccountID)
			return err
		})
		return result, err
	}
	return s.replanWeekLocked(ctx, tenantID, from, to, activityGroupID, actorAccountID)
}

func (s *InstanceLifecycleService) replanWeekLocked(ctx context.Context, tenantID int64, from, to calendar.Date, activityGroupID, actorAccountID *int64) (*timetable.ReplanWeekResult, error) {
	// Recurrence, then the grade-transition gate the materialization below
	// reads roster state under, then the days (#6232).
	if err := s.deps.RecurrenceLock.LockRecurrenceWritesThenGradeTransitions(ctx); err != nil {
		return nil, &ports.ScheduleError{Op: "replan week: lock recurrence", Err: err}
	}
	// Lock every day of the window, ascending, BEFORE snapshotting: the
	// re-plan regenerates exactly the rows the day-wide staffing saves write,
	// so a save committing between snapshot and delete would be lost (#1840).
	if err := s.LockDeviationDays(ctx, tenantID, from, to); err != nil {
		return nil, &ports.ScheduleError{Op: "replan week: lock days", Err: err}
	}
	// Regenerate every occurrence from the (possibly just edited) template
	// and reapply only the deviation fields; freezing a deviated row would
	// keep stale template values beside a fresh duplicate.
	snapshots, occurrences, err := s.snapshotDeviations(ctx, from, to, activityGroupID)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "replan week: snapshot deviations", Err: err}
	}
	// Legacy weekend occurrences are deleted too: the materialization no
	// longer recreates them.
	fromDate, toDate := calendar.Date(from), calendar.Date(to)
	deleted, err := deletePlannedInstancesInWindow(ctx, s.deps.InstanceRepo.Plans, fromDate, &toDate, activityGroupID, false)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "replan week: delete planned", Err: err}
	}
	mat, err := s.deps.Materialization.MaterializeForTenant(ctx, from, to, timetable.MaterializationSourceManual)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "replan week: materialize", Err: err}
	}
	reapplied, err := s.reapplyDeviations(ctx, snapshots, occurrences, nil, actorAccountID)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "replan week: reapply deviations", Err: err}
	}
	// Deletions bypass the CRUD announcements; the materializer announces
	// its own inserts.
	if deleted > 0 {
		s.broadcastPlannedInstanceChanged(ctx, "replan_week")
	}
	s.getLogger().Info("replan week completed",
		slog.Int64("tenant_id", tenantID),
		slog.String("from", from.String()),
		slog.String("to", to.String()),
		slog.Int64("deleted_instances", deleted),
		slog.Int("instances_created", mat.InstancesCreated),
		slog.Int("deviations_snapshotted", overriddenCount(snapshots)),
		slog.Int("deviations_reapplied", reapplied),
	)
	return &timetable.ReplanWeekResult{From: from, To: to, DeletedInstances: int(deleted), Materialization: mat}, nil
}

// deviationSnapshot captures the Vertretungsplan overrides (#1840) and the
// per-occurrence Personalbedarf pin (#1839) of one planned template-backed
// occurrence, keyed by the materializer's slot (group, date, start time).
type deviationSnapshot struct {
	date             calendar.Date
	activityGroupID  int64
	startTime        string // "15:04:05", for multi-slot disambiguation
	understaffedAck  bool
	understaffedNote *string
	// requiredStaff is a deliberate single-occurrence pin: materialized rows
	// leave the column NULL and inherit the template at read time.
	requiredStaff *int
	// absentPlanned: planned (non-substitute) rows marked absent.
	absentPlanned []snapshotAbsence
	// substitutes: substitute rows to recreate.
	substitutes []snapshotSubstitute
	// presentPlanned: planned rows that were present. The materializer marks
	// a person on a full sick day absent; a row that was present on such a
	// day was restored by hand and stays present (#6292).
	presentPlanned []int64
}

// snapshotAbsence keeps the sick provenance with the absence, so a
// regenerated row still belongs to its sick report (#6292).
type snapshotAbsence struct {
	staffID       int64
	reason        *string
	sickAbsenceID *int64
}

type snapshotSubstitute struct {
	staffID       int64
	roomID        *int64
	isPrimary     bool
	isAbsent      bool
	reason        *string
	sickAbsenceID *int64
}

// groupDay keys occurrences by their (activity group, date).
type groupDay struct {
	activityGroupID int64
	date            calendar.Date
}

// snapshotDeviations records every reappliable override on the planned
// template-backed occurrences a re-plan is about to delete. occurrences
// counts ALL of them per (group, date), overridden or not: that original
// cardinality is what tells a moved sole occurrence from a multi-slot day.
func (s *InstanceLifecycleService) snapshotDeviations(ctx context.Context, from, to calendar.Date, activityGroupID *int64) ([]deviationSnapshot, map[groupDay]int, error) {
	instances, err := instancesBetween(ctx, s.deps.InstanceRepo.Reads, calendar.Date(from), calendar.Date(to))
	if err != nil {
		return nil, nil, err
	}
	// A weekend that follows Friday's plan is rematerialized (#3921), so its
	// deviations are reapplied like a weekday's.
	follows, err := WeekendFollowsFridayIn(ctx, from, to)
	if err != nil {
		return nil, nil, &ports.ScheduleError{Op: "re-plan: snapshot deviations", Err: err}
	}
	occurrences := make(map[groupDay]int)
	eligible := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	for _, inst := range instances {
		if !replannableOccurrence(inst, activityGroupID, follows) {
			continue
		}
		occurrences[groupDay{*inst.ActivityGroupID, calendar.Date(inst.Date)}]++
		eligible = append(eligible, inst)
	}
	staffRows, err := staffOfInstances(ctx, s.deps.InstanceStaffRepo, instanceRowIDs(eligible))
	if err != nil {
		return nil, nil, err
	}
	// The caller deletes these occurrences and then plans their people
	// again (materialization, reapplied substitutes). Offboarding holds a
	// staff row while it locks that person's blocks, history included, so
	// the live people of the occurrences are locked now, before the delete
	// holds their rows (#6583, tenant.LockStaffRows).
	staff, err := s.liveStaffOf(ctx, staffRows, eligible)
	if err != nil {
		return nil, nil, err
	}
	if err := lockStaffRows(ctx, s.deps.InstanceStaffRepo, staff); err != nil {
		return nil, nil, err
	}
	staffByInstance := IndexInstanceStaffRows(staffRows)
	snapshots := make([]deviationSnapshot, 0)
	for _, inst := range eligible {
		snap := snapshotOccurrence(inst, staffByInstance[inst.ID])
		if snap.overridden() || len(snap.presentPlanned) > 0 {
			snapshots = append(snapshots, snap)
		}
	}
	return snapshots, occurrences, nil
}

// liveStaffOf lists the live people staffed on the occurrences. Offboarding
// removed a retired person's rows from today on, so those people are live;
// a past occurrence may still name a retired person, who is left out (only a
// live person can be offboarded, and only then close a cycle).
func (s *InstanceLifecycleService) liveStaffOf(ctx context.Context, rows []*scheduleModel.InstanceStaff, instances []*scheduleModel.ActivityInstance) ([]int64, error) {
	today := calendar.TodayDate()
	past := make(map[int64]bool, len(instances))
	for _, inst := range instances {
		past[inst.ID] = calendar.Date(inst.Date).Before(today)
	}
	staff := make([]int64, 0, len(rows))
	var history []int64
	for _, row := range rows {
		if past[row.InstanceID] {
			history = append(history, row.StaffID)
		} else {
			staff = append(staff, row.StaffID)
		}
	}
	if len(history) == 0 {
		return staff, nil
	}
	live, err := s.deps.StaffRepo.FindByIDs(ctx, history)
	if err != nil {
		return nil, &ports.ScheduleError{Op: "re-plan: load staff of past occurrences", Err: err}
	}
	for _, staffID := range history {
		if live[staffID] != nil {
			staff = append(staff, staffID)
		}
	}
	return staff, nil
}

func replannableOccurrence(inst *scheduleModel.ActivityInstance, activityGroupID *int64, weekendFollowsFriday bool) bool {
	if !weekendFollowsFriday && (inst.Date.Weekday() == time.Saturday || inst.Date.Weekday() == time.Sunday) {
		return false
	}
	if inst.Status != scheduleModel.InstanceStatusPlanned || inst.IsSpontaneous || inst.ActivityGroupID == nil {
		return false
	}
	return activityGroupID == nil || *inst.ActivityGroupID == *activityGroupID
}

func snapshotOccurrence(inst *scheduleModel.ActivityInstance, staffRows []*scheduleModel.InstanceStaff) deviationSnapshot {
	snap := deviationSnapshot{
		date:             calendar.Date(inst.Date),
		activityGroupID:  *inst.ActivityGroupID,
		startTime:        FormatTimeOfDay(inst.StartTime),
		understaffedAck:  inst.UnderstaffedAck,
		understaffedNote: inst.UnderstaffedNote,
		requiredStaff:    inst.RequiredStaff,
	}
	for _, row := range staffRows {
		switch {
		case row.IsSubstitute:
			snap.substitutes = append(snap.substitutes, snapshotSubstitute{
				staffID: row.StaffID, roomID: row.RoomID, isPrimary: row.IsPrimary, isAbsent: row.IsAbsent, reason: row.AbsenceReason,
				sickAbsenceID: row.SickAbsenceID,
			})
		case row.IsAbsent:
			snap.absentPlanned = append(snap.absentPlanned, snapshotAbsence{
				staffID: row.StaffID, reason: row.AbsenceReason, sickAbsenceID: row.SickAbsenceID,
			})
		default:
			snap.presentPlanned = append(snap.presentPlanned, row.StaffID)
		}
	}
	return snap
}

// overriddenCount counts the snapshots that carry a deviation; snapshots of
// present rows alone are no deviation.
func overriddenCount(snapshots []deviationSnapshot) int {
	count := 0
	for _, snap := range snapshots {
		if snap.overridden() {
			count++
		}
	}
	return count
}

// overridden reports whether the occurrence carries a deviation to reapply.
// The present rows alone are no deviation: such a snapshot is neither
// counted nor recorded as dropped when its occurrence no longer materializes.
func (snap deviationSnapshot) overridden() bool {
	return snap.understaffedAck || snap.requiredStaff != nil || len(snap.absentPlanned) > 0 || len(snap.substitutes) > 0
}

// reapplyDeviations reattaches each snapshotted override onto the freshly
// materialized occurrence and reports how many it reapplied. A snapshot
// whose occurrence no longer materializes is dropped and the loss recorded
// in the Änderungsprotokoll (#1886); successful reapplies are not logged.
func (s *InstanceLifecycleService) reapplyDeviations(
	ctx context.Context, snapshots []deviationSnapshot, occurrences map[groupDay]int, targetActivityGroupID, actorAccountID *int64,
) (int, error) {
	matches, err := s.matchRegeneratedInstances(ctx, snapshots, occurrences, targetActivityGroupID)
	if err != nil {
		return 0, err
	}
	matched := make([]*scheduleModel.ActivityInstance, 0, len(matches))
	for _, instance := range matches {
		if instance != nil {
			matched = append(matched, instance)
		}
	}
	staffRows, err := staffOfInstances(ctx, s.deps.InstanceStaffRepo, instanceRowIDs(matched))
	if err != nil {
		return 0, err
	}
	staffByInstance := IndexInstanceStaffRows(staffRows)
	reapplied := 0
	for i, snap := range snapshots {
		var rows []*scheduleModel.InstanceStaff
		if matches[i] != nil {
			rows = staffByInstance[matches[i].ID]
		}
		if err := s.reapplyMatchedSnapshot(ctx, snap, matches[i], rows, targetActivityGroupID, actorAccountID); err != nil {
			return reapplied, err
		}
		if matches[i] != nil && snap.overridden() {
			reapplied++
		}
	}
	return reapplied, nil
}

// reapplyMatchedSnapshot reapplies one snapshot onto its regenerated
// occurrence. Without one, a snapshot carrying a deviation is recorded as
// dropped; one holding only present rows has nothing to lose.
func (s *InstanceLifecycleService) reapplyMatchedSnapshot(
	ctx context.Context, snap deviationSnapshot, inst *scheduleModel.ActivityInstance, rows []*scheduleModel.InstanceStaff, targetActivityGroupID, actorAccountID *int64,
) error {
	if inst != nil {
		return s.reapplySnapshot(ctx, snap, inst, rows)
	}
	if !snap.overridden() {
		return nil
	}
	return s.logSnapshotDropped(ctx, snap, targetActivityGroupID, actorAccountID)
}

// reapplySnapshot writes one snapshot onto its regenerated occurrence: the
// Personalbedarf pin, the planned absences, the substitutes those absences
// still justify and the acknowledgement.
func (s *InstanceLifecycleService) reapplySnapshot(ctx context.Context, snap deviationSnapshot, inst *scheduleModel.ActivityInstance, rows []*scheduleModel.InstanceStaff) error {
	// Column-scoped: a full-row update does not round-trip TIME columns.
	if snap.requiredStaff != nil {
		inst.RequiredStaff = snap.requiredStaff
		if _, err := updateLegacyInstanceColumns(ctx, s.deps.InstanceRepo, inst, "required_staff"); err != nil {
			return err
		}
	}
	byStaff := make(map[int64]*scheduleModel.InstanceStaff, len(rows))
	for _, row := range rows {
		byStaff[row.StaffID] = row
	}
	day := calendar.Date(inst.Date)
	if err := s.keepRestoredPresence(ctx, day, snap.presentPlanned, byStaff); err != nil {
		return err
	}
	absencesReapplied, err := s.reapplyAbsences(ctx, day, snap.absentPlanned, byStaff)
	if err != nil {
		return err
	}
	if err := s.recreateSubstitutes(ctx, inst.ID, day, snap.substitutes, byStaff, absencesReapplied); err != nil {
		return err
	}
	// SetUnderstaffedAck re-reads the roster just written and refuses the
	// ack only when the block is fully staffed now; the stale ack is then
	// dropped, matching the endpoints' reconciliation.
	if snap.understaffedAck {
		if _, err := s.setUnderstaffedAck(ctx, inst.ID, true, snap.understaffedNote, nil, false); err != nil &&
			!errors.Is(err, timetable.ErrUnderstaffedAckStillStaffed) {
			return err
		}
	}
	return nil
}

// keepRestoredPresence undoes the materializer's sick mark on a row that was
// present before the re-plan: a planner had restored the person on a sick
// day (applyPresence releases the stamp), and a re-plan regenerates the
// template, it does not revisit that decision (#6292).
func (s *InstanceLifecycleService) keepRestoredPresence(ctx context.Context, day calendar.Date, staffIDs []int64, byStaff map[int64]*scheduleModel.InstanceStaff) error {
	for _, staffID := range staffIDs {
		row, ok := byStaff[staffID]
		if !ok || row.IsSubstitute || row.SickAbsenceID == nil {
			continue
		}
		copyAbsenceState(row, &scheduleModel.InstanceStaff{})
		if err := writeStaffAssignment(ctx, s.deps.InstanceStaffRepo, row, day, nil, true); err != nil {
			return err
		}
	}
	return nil
}

// reapplyAbsences marks the regenerated planned rows absent again, with the
// reason and the sick provenance they had. A row the materializer already
// marked for a sick report takes the snapshot's state, so a manual absence
// stays manual. A person no longer planned on the template has no row; the
// absence is moot.
func (s *InstanceLifecycleService) reapplyAbsences(ctx context.Context, day calendar.Date, absences []snapshotAbsence, byStaff map[int64]*scheduleModel.InstanceStaff) (int, error) {
	reapplied := 0
	for _, ab := range absences {
		row, ok := byStaff[ab.staffID]
		if !ok || row.IsSubstitute {
			continue
		}
		copyAbsenceState(row, &scheduleModel.InstanceStaff{IsAbsent: true, AbsenceReason: ab.reason, SickAbsenceID: ab.sickAbsenceID})
		if err := writeStaffAssignment(ctx, s.deps.InstanceStaffRepo, row, day, nil, true); err != nil {
			return reapplied, err
		}
		reapplied++
	}
	return reapplied, nil
}

// recreateSubstitutes recreates the snapshot's substitute rows. A substitute
// exists only to cover an absent planned position, so ACTIVE substitutes
// are capped at the absences that actually landed again: a template edit
// that removed an absent person must not leave an orphaned extra supervisor
// that overstaffs the block. Absent substitute rows are dead history, staff
// nothing and are not counted against the cap, so snapshot order cannot let
// history crowd out a live replacement (#1840). A person already on the
// regenerated block is skipped (UNIQUE(instance_id, staff_id)).
func (s *InstanceLifecycleService) recreateSubstitutes(
	ctx context.Context, instanceID int64, day calendar.Date, substitutes []snapshotSubstitute, byStaff map[int64]*scheduleModel.InstanceStaff, budget int,
) error {
	recreated := 0
	for _, sub := range substitutes {
		if _, taken := byStaff[sub.staffID]; taken {
			continue
		}
		if !sub.isAbsent {
			if recreated >= budget {
				continue
			}
			recreated++
		}
		newRow := &scheduleModel.InstanceStaff{
			InstanceID:    instanceID,
			StaffID:       sub.staffID,
			RoomID:        sub.roomID,
			IsPrimary:     sub.isPrimary,
			IsSubstitute:  true,
			IsAbsent:      sub.isAbsent,
			AbsenceReason: sub.reason,
			SickAbsenceID: sub.sickAbsenceID,
		}
		// The snapshot is the substitute's state of the same day: its absence
		// and sick stamp carry over (I-1).
		if err := writeStaffAssignment(ctx, s.deps.InstanceStaffRepo, newRow, day, nil, true); err != nil {
			return err
		}
		byStaff[sub.staffID] = newRow
	}
	return nil
}
