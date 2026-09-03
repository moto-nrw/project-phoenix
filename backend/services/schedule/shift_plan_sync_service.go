// Package schedule — the #1843 sick cascade: one sick report fans out into
// shift cancellations (Dienstplan) and per-day block absences (Betreuungsplan),
// and deleting the report reverses exactly the rows it stamped.
//
// Implements active.ShiftPlanSyncer (declared in services/active because that
// package cannot import this one). Both directions run inside the caller's
// tenant transaction and are FAIL-CLOSED: the first error aborts the whole
// absence write, so a half-cascaded sick report never commits.
package schedule

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/uptrace/bun"
)

// sickShiftChangeReason / sickBlockAbsenceReason are the neutral machine
// labels the cascade writes. The absence note is deliberately NOT propagated:
// plan surfaces (Dienstplan cells, block badges) are visible to the whole
// team, while the note may carry private medical detail.
const (
	sickShiftChangeReason  = "Krankheit"
	sickBlockAbsenceReason = "Krankmeldung"
)

type shiftPlanSyncService struct {
	shifts            StaffShiftService
	instances         InstanceService
	timetableData     *TimetableDataService
	shiftRepo         scheduleModel.StaffShiftRepository
	instanceStaffRepo scheduleModel.InstanceStaffRepository
	broadcaster       realtime.Broadcaster
	db                *bun.DB
	logger            *slog.Logger
	today             func() timezone.Date
}

func (s *shiftPlanSyncService) todayDate() timezone.Date {
	if s.today != nil {
		return s.today()
	}
	return timezone.TodayDate()
}

// NewShiftPlanSyncService wires the #1843 cascade. All dependencies are the
// already-constructed schedule services/repos; the result is injected into the
// staff absence service via SetShiftPlanSyncer.
func NewShiftPlanSyncService(
	shifts StaffShiftService,
	instances InstanceService,
	timetableData *TimetableDataService,
	shiftRepo scheduleModel.StaffShiftRepository,
	instanceStaffRepo scheduleModel.InstanceStaffRepository,
	broadcaster realtime.Broadcaster,
	db *bun.DB,
	logger *slog.Logger,
	today ...func() timezone.Date,
) active.ShiftPlanSyncer {
	service := &shiftPlanSyncService{
		shifts:            shifts,
		instances:         instances,
		timetableData:     timetableData,
		shiftRepo:         shiftRepo,
		instanceStaffRepo: instanceStaffRepo,
		broadcaster:       broadcaster,
		db:                db,
		logger:            logger,
	}
	if len(today) > 0 {
		service.today = today[0]
	}
	return service
}

func (s *shiftPlanSyncService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *shiftPlanSyncService) lockStaffWrites(ctx context.Context, staffID int64) error {
	if s.db == nil {
		return fmt.Errorf("sick cascade: staff lock database is not configured")
	}
	if err := LockStaffShiftWrites(ctx, s.db, staffID); err != nil {
		return fmt.Errorf("sick cascade: lock staff %d: %w", staffID, err)
	}
	return nil
}

// MarkSickForRange cancels the subject's shifts and marks their care-block
// rows absent for every full sick day. Phase order is fixed — ALL shift writes
// (per-staff advisory locks) strictly before ALL block writes (per-day locks,
// ascending) — so the two lock classes never interleave across concurrent
// requests and cannot deadlock against a parallel admin substitution save.
func (s *shiftPlanSyncService) MarkSickForRange(ctx context.Context, in active.SickCascadeInput) error {
	days := sickCascadeDays(in)
	if len(days) == 0 {
		return nil
	}
	if err := s.lockStaffWrites(ctx, in.SubjectStaffID); err != nil {
		return err
	}

	if err := s.cancelShiftsForSickDays(ctx, in, days); err != nil {
		return err
	}
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)
	if err := s.markBlocksForSickDays(ctx, in, days, activeTouched); err != nil {
		return err
	}
	s.instances.QueueActivityUpdates(ctx, activeTouched)
	s.broadcastStaffingChanged(ctx, "sick_cascade_mark")
	s.getLogger().Info("sick cascade applied",
		"staff_id", in.SubjectStaffID,
		"absence_id", in.AbsenceID,
		"date_start", in.DateStart.String(),
		"date_end", in.DateEnd.String(),
	)
	return nil
}

func (s *shiftPlanSyncService) cancelShiftsForSickDays(ctx context.Context, in active.SickCascadeInput, days []timezone.Date) error {
	shifts, err := s.shiftRepo.FindByStaffAndDateRange(ctx, in.SubjectStaffID, days[0], days[len(days)-1])
	if err != nil {
		return fmt.Errorf("sick cascade: load shifts: %w", err)
	}
	daySet := make(map[timezone.Date]bool, len(days))
	for _, d := range days {
		daySet[d] = true
	}
	reason := sickShiftChangeReason
	today := s.todayDate()
	for _, shift := range shifts {
		if !daySet[shift.Date] {
			continue // boundary half days never cascade
		}
		if shift.Date.Before(today) {
			continue // completed planning and time-tracking history is immutable
		}
		if shift.Cancelled {
			continue // an admin already cancelled it — keep that reason and its covers
		}
		if shift.OriginShiftID != nil {
			// The sick person covers a colleague's cancelled shift. Cancelling
			// a cover is invalid and deleting it would destroy admin work, so
			// leave it for the admin to re-plan.
			s.getLogger().Warn("sick cascade left a replacement shift in place",
				"shift_id", shift.ID,
				"staff_id", shift.StaffID,
				"date", shift.Date.String(),
			)
			continue
		}
		result, err := s.shifts.ApplyCancellation(ctx, CancelShiftInput{
			ShiftID:      shift.ID,
			Cancelled:    true,
			ChangeReason: &reason,
			ActorStaffID: in.ActorStaffID,
		})
		if err != nil {
			return fmt.Errorf("sick cascade: cancel shift %d: %w", shift.ID, err)
		}
		result.Shift.SickAbsenceID = &in.AbsenceID
		if _, err := s.shiftRepo.UpdateColumns(ctx, result.Shift, "sick_absence_id"); err != nil {
			return fmt.Errorf("sick cascade: stamp shift %d: %w", shift.ID, err)
		}
	}
	return nil
}

func (s *shiftPlanSyncService) markBlocksForSickDays(ctx context.Context, in active.SickCascadeInput, days []timezone.Date, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	// Past days are never touched: a completed/historical instance records
	// what actually happened (mirrors the deviations endpoints' past guard).
	today := s.todayDate()
	reason := sickBlockAbsenceReason
	for _, d := range days {
		if d.Before(today) {
			continue
		}
		if err := s.markBlocksForSickDay(ctx, in, d, &reason, activeTouched); err != nil {
			return err
		}
	}
	return nil
}

func (s *shiftPlanSyncService) markBlocksForSickDay(ctx context.Context, in active.SickCascadeInput, day timezone.Date, reason *string, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	if err := s.timetableData.AcquireSubstituteDayLock(ctx, day); err != nil {
		return fmt.Errorf("sick cascade: day lock %s: %w", day.String(), err)
	}
	rows, err := s.timetableData.GetInstanceStaffByStaffAndDate(ctx, in.SubjectStaffID, day)
	if err != nil {
		return fmt.Errorf("sick cascade: load assignments %s: %w", day.String(), err)
	}
	instances, err := s.timetableData.GetActivityInstances(ctx, instanceStaffInstanceIDs(rows))
	if err != nil {
		return fmt.Errorf("sick cascade: load instances for %s: %w", day.String(), err)
	}
	instancesByID := indexActivityInstances(instances)
	if missingID := missingActivityInstanceID(rows, instancesByID); missingID > 0 {
		return fmt.Errorf("sick cascade: load instance %d: %w", missingID, modelBase.ErrNotFound)
	}
	for _, row := range rows {
		if row.IsAbsent {
			continue // already absent (manual deviation or overlapping report)
		}
		instance := instancesByID[row.InstanceID]
		if instance == nil || !sickCascadePlannable(instance) {
			continue
		}
		if err := s.instances.ApplySickAbsence(ctx, row, instance, reason, in.AbsenceID, in.ActorAccountID, activeTouched); err != nil {
			return fmt.Errorf("sick cascade: mark block %d: %w", row.InstanceID, err)
		}
	}
	return nil
}

// ClearSickForRange reverses what MarkSickForRange stamped for this absence.
// Rows that meanwhile received admin work are skipped but released: a shift
// with replacements stays cancelled, a block with an active substitute stays
// absent — in both cases only the provenance stamp is cleared so the deleted
// report stops owning them, and the admin resolves the rest manually.
func (s *shiftPlanSyncService) ClearSickForRange(ctx context.Context, in active.SickCascadeInput) error {
	if err := s.lockSickReversalStaffWrites(ctx, in); err != nil {
		return err
	}
	if err := s.reactivateStampedShifts(ctx, in, nil); err != nil {
		return err
	}
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)
	if err := s.clearStampedBlocks(ctx, in, nil, activeTouched); err != nil {
		return err
	}
	s.instances.QueueActivityUpdates(ctx, activeTouched)
	s.broadcastStaffingChanged(ctx, "sick_cascade_clear")
	s.getLogger().Info("sick cascade cleared",
		"staff_id", in.SubjectStaffID,
		"absence_id", in.AbsenceID,
	)
	return nil
}

// ReconcileSickRange applies only the days removed from or added to an edited
// full-day sick report. The staff lock is acquired before either side reads
// shifts, and every affected future day lock is acquired in ascending order
// before block writes, so the two directions cannot deadlock each other.
func (s *shiftPlanSyncService) ReconcileSickRange(ctx context.Context, before, after active.SickCascadeInput) error {
	if before.SubjectStaffID != after.SubjectStaffID || before.AbsenceID != after.AbsenceID {
		return fmt.Errorf("sick reconcile: before and after must identify the same absence")
	}
	removed, added := sickCascadeDayDifference(before, after)
	if len(removed) == 0 && len(added) == 0 {
		return nil
	}
	if err := s.lockSickReversalStaffWrites(ctx, before); err != nil {
		return err
	}
	if err := s.acquireCascadeDayLocks(ctx, unionDateSets(removed, added)); err != nil {
		return err
	}
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)
	if err := s.reconcileRemovedSickDays(ctx, before, removed, activeTouched); err != nil {
		return err
	}
	if err := s.reconcileAddedSickDays(ctx, after, added, activeTouched); err != nil {
		return err
	}
	s.instances.QueueActivityUpdates(ctx, activeTouched)
	s.broadcastStaffingChanged(ctx, "sick_cascade_reconcile")
	return nil
}

func (s *shiftPlanSyncService) broadcastStaffingChanged(ctx context.Context, source string) {
	broadcastStaffingChanged(ctx, s.broadcaster, s.getLogger(), source)
}

// lockSickReversalStaffWrites discovers every replacement owner attached to a
// shift stamped by this sick report, then acquires the complete staff lock set
// in the same global order used by ApplyCancellation. Taking the subject lock
// first and discovering a lower-ID replacement later creates the inverse lock
// order and can deadlock against a concurrent replacement edit.
func (s *shiftPlanSyncService) lockSickReversalStaffWrites(ctx context.Context, in active.SickCascadeInput) error {
	shifts, err := s.shiftRepo.List(ctx, map[string]any{"sick_absence_id": in.AbsenceID})
	if err != nil {
		return fmt.Errorf("sick clear: discover stamped shifts: %w", err)
	}
	staffIDs := []int64{in.SubjectStaffID}
	covers, err := s.shiftRepo.FindByOriginShiftIDs(ctx, staffShiftIDs(shifts))
	if err != nil {
		return fmt.Errorf("sick clear: discover stamped shift covers: %w", err)
	}
	for _, cover := range covers {
		staffIDs = append(staffIDs, cover.StaffID)
	}
	sort.Slice(staffIDs, func(i, j int) bool { return staffIDs[i] < staffIDs[j] })
	var previous int64
	for _, staffID := range staffIDs {
		if staffID <= 0 || staffID == previous {
			continue
		}
		if err := s.lockStaffWrites(ctx, staffID); err != nil {
			return err
		}
		previous = staffID
	}
	return nil
}

func staffShiftIDs(shifts []*scheduleModel.StaffShift) []int64 {
	ids := make([]int64, 0, len(shifts))
	for _, shift := range shifts {
		ids = append(ids, shift.ID)
	}
	return ids
}

func (s *shiftPlanSyncService) reconcileRemovedSickDays(ctx context.Context, before active.SickCascadeInput, removed map[timezone.Date]bool, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	if len(removed) == 0 {
		return nil
	}
	if err := s.reactivateStampedShifts(ctx, before, removed); err != nil {
		return err
	}
	return s.clearStampedBlocks(ctx, before, removed, activeTouched)
}

func (s *shiftPlanSyncService) reconcileAddedSickDays(ctx context.Context, after active.SickCascadeInput, added map[timezone.Date]bool, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	if len(added) == 0 {
		return nil
	}
	days := sortedDateSet(added)
	if err := s.cancelShiftsForSickDays(ctx, after, days); err != nil {
		return err
	}
	return s.markBlocksForSickDays(ctx, after, days, activeTouched)
}

func (s *shiftPlanSyncService) acquireCascadeDayLocks(ctx context.Context, days map[timezone.Date]bool) error {
	today := s.todayDate()
	for _, day := range sortedDateSet(days) {
		if day.Before(today) {
			continue
		}
		if err := s.timetableData.AcquireSubstituteDayLock(ctx, day); err != nil {
			return fmt.Errorf("sick reconcile: day lock %s: %w", day.String(), err)
		}
	}
	return nil
}

func (s *shiftPlanSyncService) reactivateStampedShifts(ctx context.Context, in active.SickCascadeInput, onlyDays map[timezone.Date]bool) error {
	shifts, err := s.shiftRepo.List(ctx, map[string]any{"sick_absence_id": in.AbsenceID})
	if err != nil {
		return fmt.Errorf("sick clear: load stamped shifts: %w", err)
	}
	today := s.todayDate()
	for _, shift := range shifts {
		if onlyDays != nil && !onlyDays[shift.Date] {
			continue
		}
		if shift.Date.Before(today) {
			// Keep the historical cancellation intact, but release the deleted
			// or edited report's provenance stamp just as past block rows do.
			if err := s.releaseStampedShift(ctx, shift, 0); err != nil {
				return err
			}
			continue
		}
		if err := s.reactivateStampedShift(ctx, in, shift); err != nil {
			return err
		}
	}
	return nil
}

func (s *shiftPlanSyncService) reactivateStampedShift(ctx context.Context, in active.SickCascadeInput, shift *scheduleModel.StaffShift) error {
	covers, err := s.shiftRepo.FindByOriginShiftID(ctx, shift.ID)
	if err != nil {
		return fmt.Errorf("sick clear: load covers of shift %d: %w", shift.ID, err)
	}
	if !shift.Cancelled || len(covers) > 0 {
		return s.releaseStampedShift(ctx, shift, len(covers))
	}
	// Reactivation clears sick_absence_id and change_reason via the ordinary
	// update path and re-checks overlap against the freed window.
	if _, err := s.shifts.ApplyCancellation(ctx, CancelShiftInput{
		ShiftID: shift.ID, Cancelled: false, ActorStaffID: in.ActorStaffID,
	}); err != nil {
		return fmt.Errorf("sick clear: reactivate shift %d: %w", shift.ID, err)
	}
	return nil
}

func (s *shiftPlanSyncService) releaseStampedShift(ctx context.Context, shift *scheduleModel.StaffShift, replacementCount int) error {
	shift.SickAbsenceID = nil
	if _, err := s.shiftRepo.UpdateColumns(ctx, shift, "sick_absence_id"); err != nil {
		return fmt.Errorf("sick clear: release shift %d: %w", shift.ID, err)
	}
	if replacementCount > 0 {
		s.getLogger().Info("sick clear kept a replaced shift cancelled",
			"shift_id", shift.ID,
			"staff_id", shift.StaffID,
			"replacements", replacementCount,
		)
	}
	return nil
}

func (s *shiftPlanSyncService) clearStampedBlocks(ctx context.Context, in active.SickCascadeInput, onlyDays map[timezone.Date]bool, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	rows, err := s.loadStampedBlockRows(ctx, in.AbsenceID)
	if err != nil || len(rows) == 0 {
		return err
	}
	byDay, releaseOnly, err := s.classifyStampedBlockRows(ctx, rows, onlyDays)
	if err != nil {
		return err
	}
	if err := s.releaseSickBlockStamps(ctx, releaseOnly); err != nil {
		return err
	}
	for _, day := range sortedStampedBlockDays(byDay) {
		if err := s.clearStampedBlocksForDay(ctx, in, day, byDay[day], activeTouched); err != nil {
			return err
		}
	}
	return nil
}

type stampedSickBlockRow struct {
	row      *scheduleModel.InstanceStaff
	instance *scheduleModel.ActivityInstance
}

func (s *shiftPlanSyncService) loadStampedBlockRows(ctx context.Context, absenceID int64) ([]*scheduleModel.InstanceStaff, error) {
	listOptions := modelBase.NewQueryOptions()
	listOptions.Filter.Equal("sick_absence_id", absenceID)
	rows, err := s.instanceStaffRepo.List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("sick clear: load stamped rows: %w", err)
	}
	return rows, nil
}

func (s *shiftPlanSyncService) classifyStampedBlockRows(ctx context.Context, rows []*scheduleModel.InstanceStaff, onlyDays map[timezone.Date]bool) (map[timezone.Date][]stampedSickBlockRow, []*scheduleModel.InstanceStaff, error) {
	today := s.todayDate()
	byDay := make(map[timezone.Date][]stampedSickBlockRow)
	var releaseOnly []*scheduleModel.InstanceStaff
	instances, err := s.timetableData.GetActivityInstances(ctx, instanceStaffInstanceIDs(rows))
	if err != nil {
		return nil, nil, fmt.Errorf("sick clear: load stamped instances: %w", err)
	}
	instancesByID := indexActivityInstances(instances)
	if missingID := missingActivityInstanceID(rows, instancesByID); missingID > 0 {
		return nil, nil, fmt.Errorf("sick clear: load instance %d: %w", missingID, modelBase.ErrNotFound)
	}
	for _, row := range rows {
		instance := instancesByID[row.InstanceID]
		if instance != nil && onlyDays != nil && !onlyDays[instance.Date] {
			continue
		}
		// Past days stay as recorded history; rows a manual edit already
		// restored, and terminal instances, only shed their stamp.
		if instance == nil || !row.IsAbsent || instance.Date.Before(today) || !sickCascadePlannable(instance) {
			releaseOnly = append(releaseOnly, row)
			continue
		}
		byDay[instance.Date] = append(byDay[instance.Date], stampedSickBlockRow{row: row, instance: instance})
	}
	return byDay, releaseOnly, nil
}

func instanceStaffInstanceIDs(rows []*scheduleModel.InstanceStaff) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.InstanceID)
	}
	return ids
}

func indexActivityInstances(instances []*scheduleModel.ActivityInstance) map[int64]*scheduleModel.ActivityInstance {
	byID := make(map[int64]*scheduleModel.ActivityInstance, len(instances))
	for _, instance := range instances {
		byID[instance.ID] = instance
	}
	return byID
}

func missingActivityInstanceID(rows []*scheduleModel.InstanceStaff, instances map[int64]*scheduleModel.ActivityInstance) int64 {
	for _, row := range rows {
		if instances[row.InstanceID] == nil {
			return row.InstanceID
		}
	}
	return 0
}

func (s *shiftPlanSyncService) releaseSickBlockStamps(ctx context.Context, rows []*scheduleModel.InstanceStaff) error {
	for _, row := range rows {
		row.SickAbsenceID = nil
		if _, err := s.instanceStaffRepo.UpdateColumns(ctx, row, "sick_absence_id"); err != nil {
			return fmt.Errorf("sick clear: release row %d: %w", row.ID, err)
		}
	}
	return nil
}

func sortedStampedBlockDays(byDay map[timezone.Date][]stampedSickBlockRow) []timezone.Date {
	dates := make([]timezone.Date, 0, len(byDay))
	for d := range byDay {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	return dates
}

func (s *shiftPlanSyncService) clearStampedBlocksForDay(ctx context.Context, in active.SickCascadeInput, day timezone.Date, entries []stampedSickBlockRow, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	if err := s.timetableData.AcquireSubstituteDayLock(ctx, day); err != nil {
		return fmt.Errorf("sick clear: day lock %s: %w", day.String(), err)
	}
	for _, entry := range entries {
		hasSubstitute, err := s.instanceHasActiveSubstitute(ctx, entry.row.InstanceID)
		if err != nil {
			return err
		}
		if hasSubstitute {
			// An admin covered this block for the sick person; restoring
			// presence would silently overstaff it. Release the stamp and
			// keep the absence for the admin to resolve.
			entry.row.SickAbsenceID = nil
			if _, err := s.instanceStaffRepo.UpdateColumns(ctx, entry.row, "sick_absence_id"); err != nil {
				return fmt.Errorf("sick clear: release row %d: %w", entry.row.ID, err)
			}
			s.getLogger().Info("sick clear kept a substituted block absent",
				"instance_id", entry.row.InstanceID,
				"staff_id", entry.row.StaffID,
			)
			continue
		}
		if err := s.instances.ClearSickAbsence(ctx, entry.row, entry.instance, in.AbsenceID, in.ActorAccountID, activeTouched); err != nil {
			return fmt.Errorf("sick clear: clear block %d: %w", entry.row.InstanceID, err)
		}
		if err := s.instances.ClearUnderstaffedAckIfStaffed(ctx, entry.row.InstanceID, in.ActorAccountID); err != nil {
			return fmt.Errorf("sick clear: reconcile understaffed acknowledgement for block %d: %w", entry.row.InstanceID, err)
		}
	}
	return nil
}

// ReassignSickStamps re-points every stamp from one absence id to another
// (overlap merge: the secondary absence row is deleted, the surviving primary
// takes over its plan effects, #1843).
func (s *shiftPlanSyncService) ReassignSickStamps(ctx context.Context, fromAbsenceID, toAbsenceID int64) error {
	shifts, err := s.shiftRepo.List(ctx, map[string]any{"sick_absence_id": fromAbsenceID})
	if err != nil {
		return fmt.Errorf("sick reassign: load stamped shifts: %w", err)
	}
	for _, shift := range shifts {
		shift.SickAbsenceID = &toAbsenceID
		if _, err := s.shiftRepo.UpdateColumns(ctx, shift, "sick_absence_id"); err != nil {
			return fmt.Errorf("sick reassign: shift %d: %w", shift.ID, err)
		}
	}
	listOptions := modelBase.NewQueryOptions()
	listOptions.Filter.Equal("sick_absence_id", fromAbsenceID)
	rows, err := s.instanceStaffRepo.List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("sick reassign: load stamped rows: %w", err)
	}
	for _, row := range rows {
		row.SickAbsenceID = &toAbsenceID
		if _, err := s.instanceStaffRepo.UpdateColumns(ctx, row, "sick_absence_id"); err != nil {
			return fmt.Errorf("sick reassign: row %d: %w", row.ID, err)
		}
	}
	return nil
}

func (s *shiftPlanSyncService) instanceHasActiveSubstitute(ctx context.Context, instanceID int64) (bool, error) {
	allRows, err := s.timetableData.GetInstanceStaff(ctx, instanceID)
	if err != nil {
		return false, fmt.Errorf("sick clear: load instance staff %d: %w", instanceID, err)
	}
	for _, r := range allRows {
		if r.IsSubstitute && !r.IsAbsent {
			return true, nil
		}
	}
	return false, nil
}

// sickCascadeDays expands the absence range into the full sick days, skipping
// half boundary days — a half sick day never cascades (#1843 product rule).
func sickCascadeDays(in active.SickCascadeInput) []timezone.Date {
	var days []timezone.Date
	for d := in.DateStart; !d.After(in.DateEnd); d = d.AddDays(1) {
		if in.SkipStartDay && d == in.DateStart {
			continue
		}
		if in.SkipEndDay && d == in.DateEnd {
			continue
		}
		days = append(days, d)
	}
	return days
}

func sickCascadeDayDifference(before, after active.SickCascadeInput) (removed, added map[timezone.Date]bool) {
	beforeDays := dateSet(sickCascadeDays(before))
	afterDays := dateSet(sickCascadeDays(after))
	removed = make(map[timezone.Date]bool)
	added = make(map[timezone.Date]bool)
	for day := range beforeDays {
		if !afterDays[day] {
			removed[day] = true
		}
	}
	for day := range afterDays {
		if !beforeDays[day] {
			added[day] = true
		}
	}
	return removed, added
}

func dateSet(days []timezone.Date) map[timezone.Date]bool {
	set := make(map[timezone.Date]bool, len(days))
	for _, day := range days {
		set[day] = true
	}
	return set
}

func unionDateSets(left, right map[timezone.Date]bool) map[timezone.Date]bool {
	union := make(map[timezone.Date]bool, len(left)+len(right))
	for day := range left {
		union[day] = true
	}
	for day := range right {
		union[day] = true
	}
	return union
}

func sortedDateSet(set map[timezone.Date]bool) []timezone.Date {
	days := make([]timezone.Date, 0, len(set))
	for day := range set {
		days = append(days, day)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	return days
}

// sickCascadePlannable mirrors the deviations endpoints' rule: only planned or
// currently active instances take staffing deviations.
func sickCascadePlannable(instance *scheduleModel.ActivityInstance) bool {
	return instance.Status == scheduleModel.InstanceStatusPlanned ||
		instance.Status == scheduleModel.InstanceStatusActive
}
