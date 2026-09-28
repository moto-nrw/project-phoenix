package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ReplanWeek deletes the planned, template-backed blocks of [from, to] for
// the current tenant, re-materializes the window and reapplies the
// Vertretungsplan overrides. Everything else survives. A non-nil
// activityGroupID narrows the delete to one template; the insert-only
// materialization still covers the whole window.
func (s *InstanceLifecycleService) ReplanWeek(ctx context.Context, from, to timezone.Date, activityGroupID, actorAccountID *int64) (*timetable.ReplanWeekResult, error) {
	if to.Before(from) {
		return nil, &ScheduleError{Op: "replan week: validate window", Err: errors.New("to_date must not be before from_date")}
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		// Without a tenant the delete would silently match nothing; fail
		// fast instead of no-oping the admin action.
		return nil, &ScheduleError{Op: "replan week", Err: errors.New("no tenant in context")}
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		var result *timetable.ReplanWeekResult
		err := tenant.WithTenantTx(ctx, s.deps.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			var err error
			result, err = s.ReplanWeek(txCtx, from, to, activityGroupID, actorAccountID)
			return err
		})
		return result, err
	}
	return s.replanWeekLocked(ctx, tenantID, from, to, activityGroupID, actorAccountID)
}

func (s *InstanceLifecycleService) replanWeekLocked(ctx context.Context, tenantID int64, from, to timezone.Date, activityGroupID, actorAccountID *int64) (*timetable.ReplanWeekResult, error) {
	if err := s.deps.RecurrenceLock.LockRecurrenceWrites(ctx); err != nil {
		return nil, &ScheduleError{Op: "replan week: lock recurrence", Err: err}
	}
	// Lock every day of the window, ascending, BEFORE snapshotting: the
	// re-plan regenerates exactly the rows the day-wide staffing saves write,
	// so a save committing between snapshot and delete would be lost (#1840).
	if err := s.acquireSubstituteDayLocks(ctx, tenantID, from, to); err != nil {
		return nil, &ScheduleError{Op: "replan week: lock days", Err: err}
	}
	// Regenerate every occurrence from the (possibly just edited) template
	// and reapply only the deviation fields; freezing a deviated row would
	// keep stale template values beside a fresh duplicate.
	snapshots, occurrences, err := s.snapshotDeviations(ctx, from, to, activityGroupID)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: snapshot deviations", Err: err}
	}
	// Legacy weekend occurrences are deleted too: the materialization no
	// longer recreates them.
	fromDate, toDate := scheduleModel.Date(from), scheduleModel.Date(to)
	deleted, err := s.deps.InstanceRepo.DeletePlannedNonSpontaneousInWindow(ctx, fromDate, &toDate, activityGroupID, false)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: delete planned", Err: err}
	}
	mat, err := s.deps.Materialization.MaterializeForTenant(ctx, from, to, timetable.MaterializationSourceManual)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: materialize", Err: err}
	}
	reapplied, err := s.reapplyDeviations(ctx, snapshots, occurrences, nil, actorAccountID)
	if err != nil {
		return nil, &ScheduleError{Op: "replan week: reapply deviations", Err: err}
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
		slog.Int("deviations_snapshotted", len(snapshots)),
		slog.Int("deviations_reapplied", reapplied),
	)
	return &timetable.ReplanWeekResult{From: from, To: to, DeletedInstances: int(deleted), Materialization: mat}, nil
}

// deviationSnapshot captures the Vertretungsplan overrides (#1840) and the
// per-occurrence Personalbedarf pin (#1839) of one planned template-backed
// occurrence, keyed by the materializer's slot (group, date, start time).
type deviationSnapshot struct {
	date             timezone.Date
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
}

type snapshotAbsence struct {
	staffID int64
	reason  *string
}

type snapshotSubstitute struct {
	staffID   int64
	roomID    *int64
	isPrimary bool
	isAbsent  bool
	reason    *string
}

// groupDay keys occurrences by their (activity group, date).
type groupDay struct {
	activityGroupID int64
	date            timezone.Date
}

// snapshotDeviations records every reappliable override on the planned
// template-backed occurrences a re-plan is about to delete. occurrences
// counts ALL of them per (group, date), overridden or not: that original
// cardinality is what tells a moved sole occurrence from a multi-slot day.
func (s *InstanceLifecycleService) snapshotDeviations(ctx context.Context, from, to timezone.Date, activityGroupID *int64) ([]deviationSnapshot, map[groupDay]int, error) {
	instances, err := s.deps.InstanceRepo.FindByTenantAndDateRange(ctx, scheduleModel.Date(from), scheduleModel.Date(to))
	if err != nil {
		return nil, nil, err
	}
	occurrences := make(map[groupDay]int)
	eligible := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	for _, inst := range instances {
		if !replannableOccurrence(inst, activityGroupID) {
			continue
		}
		occurrences[groupDay{*inst.ActivityGroupID, timezone.Date(inst.Date)}]++
		eligible = append(eligible, inst)
	}
	staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceIDs(ctx, instanceRowIDs(eligible))
	if err != nil {
		return nil, nil, err
	}
	staffByInstance := indexInstanceStaffRows(staffRows)
	snapshots := make([]deviationSnapshot, 0)
	for _, inst := range eligible {
		snap := snapshotOccurrence(inst, staffByInstance[inst.ID])
		if snap.overridden() {
			snapshots = append(snapshots, snap)
		}
	}
	return snapshots, occurrences, nil
}

func replannableOccurrence(inst *scheduleModel.ActivityInstance, activityGroupID *int64) bool {
	if inst.Date.Weekday() == time.Saturday || inst.Date.Weekday() == time.Sunday {
		return false
	}
	if inst.Status != scheduleModel.InstanceStatusPlanned || inst.IsSpontaneous || inst.ActivityGroupID == nil {
		return false
	}
	return activityGroupID == nil || *inst.ActivityGroupID == *activityGroupID
}

func snapshotOccurrence(inst *scheduleModel.ActivityInstance, staffRows []*scheduleModel.InstanceStaff) deviationSnapshot {
	snap := deviationSnapshot{
		date:             timezone.Date(inst.Date),
		activityGroupID:  *inst.ActivityGroupID,
		startTime:        formatTimeOfDay(inst.StartTime),
		understaffedAck:  inst.UnderstaffedAck,
		understaffedNote: inst.UnderstaffedNote,
		requiredStaff:    inst.RequiredStaff,
	}
	for _, row := range staffRows {
		switch {
		case row.IsSubstitute:
			snap.substitutes = append(snap.substitutes, snapshotSubstitute{
				staffID: row.StaffID, roomID: row.RoomID, isPrimary: row.IsPrimary, isAbsent: row.IsAbsent, reason: row.AbsenceReason,
			})
		case row.IsAbsent:
			snap.absentPlanned = append(snap.absentPlanned, snapshotAbsence{staffID: row.StaffID, reason: row.AbsenceReason})
		}
	}
	return snap
}

// overridden reports whether the occurrence carries anything to reapply.
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
	staffRows, err := s.deps.InstanceStaffRepo.FindByInstanceIDs(ctx, instanceRowIDs(matched))
	if err != nil {
		return 0, err
	}
	staffByInstance := indexInstanceStaffRows(staffRows)
	reapplied := 0
	for i, snap := range snapshots {
		inst := matches[i]
		if inst == nil {
			if err := s.logSnapshotDropped(ctx, snap, targetActivityGroupID, actorAccountID); err != nil {
				return reapplied, err
			}
			continue
		}
		if err := s.reapplySnapshot(ctx, snap, inst, staffByInstance[inst.ID]); err != nil {
			return reapplied, err
		}
		reapplied++
	}
	return reapplied, nil
}

// reapplySnapshot writes one snapshot onto its regenerated occurrence: the
// Personalbedarf pin, the planned absences, the substitutes those absences
// still justify and the acknowledgement.
func (s *InstanceLifecycleService) reapplySnapshot(ctx context.Context, snap deviationSnapshot, inst *scheduleModel.ActivityInstance, rows []*scheduleModel.InstanceStaff) error {
	// Column-scoped: a full-row update does not round-trip TIME columns.
	if snap.requiredStaff != nil {
		inst.RequiredStaff = snap.requiredStaff
		if _, err := s.deps.InstanceRepo.UpdateColumns(ctx, inst, "required_staff"); err != nil {
			return err
		}
	}
	byStaff := make(map[int64]*scheduleModel.InstanceStaff, len(rows))
	for _, row := range rows {
		byStaff[row.StaffID] = row
	}
	absencesReapplied, err := s.reapplyAbsences(ctx, snap.absentPlanned, byStaff)
	if err != nil {
		return err
	}
	if err := s.recreateSubstitutes(ctx, inst.ID, snap.substitutes, byStaff, absencesReapplied); err != nil {
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

// reapplyAbsences marks the regenerated planned rows absent again. A person
// no longer planned on the template has no row; the absence is moot.
func (s *InstanceLifecycleService) reapplyAbsences(ctx context.Context, absences []snapshotAbsence, byStaff map[int64]*scheduleModel.InstanceStaff) (int, error) {
	reapplied := 0
	for _, ab := range absences {
		row, ok := byStaff[ab.staffID]
		if !ok || row.IsSubstitute || row.IsAbsent {
			continue
		}
		row.IsAbsent = true
		row.AbsenceReason = ab.reason
		if err := s.deps.InstanceStaffRepo.Update(ctx, row); err != nil {
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
	ctx context.Context, instanceID int64, substitutes []snapshotSubstitute, byStaff map[int64]*scheduleModel.InstanceStaff, budget int,
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
		}
		if err := s.deps.InstanceStaffRepo.Create(ctx, newRow); err != nil {
			return err
		}
		byStaff[sub.staffID] = newRow
	}
	return nil
}
