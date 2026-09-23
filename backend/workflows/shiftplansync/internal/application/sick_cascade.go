// Package application holds the shift-plan-sync workflow's behaviour: the
// #1843 sick cascade, where one sick report fans out into shift cancellations
// (Dienstplan) and per-day block absences (Betreuungsplan) and deleting the
// report reverses exactly the rows it stamped, and the schedule substitution
// moves.
//
// The cascade implements workforce.ShiftPlanSync, the port Workforce's absence
// service declares. Both directions run inside the caller's tenant transaction
// and are FAIL-CLOSED: the first error aborts the whole absence write, so a
// half-cascaded sick report never commits.
package application

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync/ports"
)

// sickShiftChangeReason / sickBlockAbsenceReason are the neutral machine
// labels the cascade writes. The absence note is deliberately NOT propagated:
// plan surfaces (Dienstplan cells, block badges) are visible to the whole
// team, while the note may carry private medical detail.
const (
	sickShiftChangeReason  = "Krankheit"
	sickBlockAbsenceReason = "Krankmeldung"
)

// TimetableRows are the Betreuungsplan rows the cascade hands to the retained
// deviation writes, and the Timetable owner's day lock of every day-wide
// staffing mutation (timetable.SubstituteDayLockKey). The composition root
// binds them to the retained repositories.
type TimetableRows interface {
	AcquireSubstituteDayLock(ctx context.Context, date timezone.Date) error
	GetInstanceStaffByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) ([]*scheduleModel.InstanceStaff, error)
	GetActivityInstancesByID(ctx context.Context, ids []int64) (map[int64]*scheduleModel.ActivityInstance, error)
	GetInstanceStaff(ctx context.Context, instanceID int64) ([]*scheduleModel.InstanceStaff, error)
}

// SickCascadeDependencies are the owner surfaces the cascade coordinates: the
// Dienstplan through the Workforce-backed port, the Betreuungsplan through the
// retained timetable planning services.
type SickCascadeDependencies struct {
	Shifts        ports.Shifts
	Instances     timetableplanning.InstanceService
	TimetableData TimetableRows
	InstanceStaff scheduleModel.InstanceStaffRepository
	Broadcaster   realtime.Broadcaster
	Logger        *slog.Logger
	// Today is the calendar day the past-day guards compare against; nil
	// means timezone.TodayDate.
	Today func() timezone.Date
}

type sickCascade struct {
	shifts            ports.Shifts
	instances         timetableplanning.InstanceService
	timetableData     TimetableRows
	instanceStaffRepo scheduleModel.InstanceStaffRepository
	broadcaster       realtime.Broadcaster
	logger            *slog.Logger
	today             func() timezone.Date
}

// NewSickCascade wires the #1843 cascade. Compose checks the dependencies and
// binds the result into Workforce's absence lifecycle.
func NewSickCascade(deps SickCascadeDependencies) workforce.ShiftPlanSync {
	return &sickCascade{
		shifts:            deps.Shifts,
		instances:         deps.Instances,
		timetableData:     deps.TimetableData,
		instanceStaffRepo: deps.InstanceStaff,
		broadcaster:       deps.Broadcaster,
		logger:            deps.Logger,
		today:             deps.Today,
	}
}

func (s *sickCascade) todayDate() timezone.Date {
	if s.today != nil {
		return s.today()
	}
	return timezone.TodayDate()
}

func (s *sickCascade) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *sickCascade) lockStaffWrites(ctx context.Context, staffID int64) error {
	if s.shifts == nil {
		return fmt.Errorf("sick cascade: shift capability is not configured")
	}
	if err := s.shifts.LockStaffShifts(ctx, staffID); err != nil {
		return fmt.Errorf("sick cascade: lock staff %d: %w", staffID, err)
	}
	return nil
}

// MarkSickForRange cancels the subject's shifts and marks their care-block
// rows absent for every full sick day. Phase order is fixed — ALL shift writes
// (per-staff advisory locks) strictly before ALL block writes (per-day locks,
// ascending) — so the two lock classes never interleave across concurrent
// requests and cannot deadlock against a parallel admin substitution save.
func (s *sickCascade) MarkSickForRange(ctx context.Context, in workforce.SickCascadeInput) error {
	report, err := parseSickReport(in)
	if err != nil {
		return err
	}
	days := sickCascadeDays(report)
	if len(days) == 0 {
		return nil
	}
	if err := s.lockStaffWrites(ctx, report.subjectStaffID); err != nil {
		return err
	}

	if err := s.cancelShiftsForSickDays(ctx, report, days); err != nil {
		return err
	}
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)
	if err := s.markBlocksForSickDays(ctx, report, days, activeTouched); err != nil {
		return err
	}
	s.instances.QueueActivityUpdates(ctx, activeTouched)
	s.broadcastStaffingChanged(ctx, "sick_cascade_mark")
	s.getLogger().Info("sick cascade applied",
		"staff_id", report.subjectStaffID,
		"absence_id", report.absenceID,
		"date_start", report.dateStart.String(),
		"date_end", report.dateEnd.String(),
	)
	return nil
}

func (s *sickCascade) cancelShiftsForSickDays(ctx context.Context, report sickReport, days []timezone.Date) error {
	shifts, err := s.shifts.ListStaffShifts(ctx, workforce.StaffShiftFilter{
		StaffID: report.subjectStaffID,
		From:    days[0].String(),
		To:      days[len(days)-1].String(),
		Order: []workforce.StaffShiftOrder{
			{Field: workforce.StaffShiftOrderDate},
			{Field: workforce.StaffShiftOrderStartTime},
		},
	})
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
		cascades, err := s.shiftTakesSickCancellation(shift, daySet, today)
		if err != nil {
			return err
		}
		if !cascades {
			continue
		}
		result, err := s.shifts.ApplyCancellation(ctx, workforce.CancelStaffShift{
			ShiftID:      shift.ID,
			Cancelled:    true,
			ChangeReason: &reason,
			ActorStaffID: report.actorStaffID,
		})
		if err != nil {
			return fmt.Errorf("sick cascade: cancel shift %d: %w", shift.ID, err)
		}
		if _, err := s.shifts.SetStaffShiftSickAbsence(ctx, result.Shift.ID, &report.absenceID); err != nil {
			return fmt.Errorf("sick cascade: stamp shift %d: %w", shift.ID, err)
		}
	}
	return nil
}

// shiftTakesSickCancellation reports whether this shift is cancelled by the
// cascade. Skipped are the boundary half days, completed planning and
// time-tracking history, a shift an admin already cancelled (its reason and
// covers stay), and a cover the sick person owns for a colleague: cancelling
// a cover is invalid and deleting it would destroy admin work, so it is left
// for the admin to re-plan.
func (s *sickCascade) shiftTakesSickCancellation(shift workforce.StaffShift, days map[timezone.Date]bool, today timezone.Date) (bool, error) {
	date, err := shiftDate(shift)
	if err != nil {
		return false, err
	}
	if !days[date] || date.Before(today) || shift.Cancelled {
		return false, nil
	}
	if shift.OriginShiftID != nil {
		s.getLogger().Warn("sick cascade left a replacement shift in place",
			"shift_id", shift.ID,
			"staff_id", shift.StaffID,
			"date", shift.Date,
		)
		return false, nil
	}
	return true, nil
}

func (s *sickCascade) markBlocksForSickDays(ctx context.Context, report sickReport, days []timezone.Date, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	// Past days are never touched: a completed/historical instance records
	// what actually happened (mirrors the deviations endpoints' past guard).
	today := s.todayDate()
	reason := sickBlockAbsenceReason
	for _, d := range days {
		if d.Before(today) {
			continue
		}
		if err := s.markBlocksForSickDay(ctx, report, d, &reason, activeTouched); err != nil {
			return err
		}
	}
	return nil
}

func (s *sickCascade) markBlocksForSickDay(ctx context.Context, report sickReport, day timezone.Date, reason *string, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	if err := s.timetableData.AcquireSubstituteDayLock(ctx, day); err != nil {
		return fmt.Errorf("sick cascade: day lock %s: %w", day.String(), err)
	}
	rows, err := s.timetableData.GetInstanceStaffByStaffAndDate(ctx, report.subjectStaffID, day)
	if err != nil {
		return fmt.Errorf("sick cascade: load assignments %s: %w", day.String(), err)
	}
	instancesByID, err := s.timetableData.GetActivityInstancesByID(ctx, instanceStaffInstanceIDs(rows))
	if err != nil {
		return fmt.Errorf("sick cascade: load instances for %s: %w", day.String(), err)
	}
	if missingID := missingActivityInstanceID(rows, instancesByID); missingID > 0 {
		return fmt.Errorf("sick cascade: load instance %d: %w", missingID, modelBase.ErrNotFound)
	}
	for _, row := range rows {
		if row.IsAbsent {
			continue // already absent (manual deviation or overlapping report)
		}
		instance := instancesByID[row.InstanceID]
		if instance == nil || !isPlannableInstance(instance) {
			continue
		}
		if err := s.instances.ApplySickAbsence(ctx, row, instance, reason, report.absenceID, report.actorAccountID, activeTouched); err != nil {
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
func (s *sickCascade) ClearSickForRange(ctx context.Context, in workforce.SickCascadeInput) error {
	// Keyed purely by the provenance stamps, so the reported range is not read
	// and a malformed one must not block the reversal.
	report := stampScope(in)
	if err := s.lockSickReversalStaffWrites(ctx, report); err != nil {
		return err
	}
	if err := s.reactivateStampedShifts(ctx, report, nil); err != nil {
		return err
	}
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)
	if err := s.clearStampedBlocks(ctx, report, nil, activeTouched); err != nil {
		return err
	}
	s.instances.QueueActivityUpdates(ctx, activeTouched)
	s.broadcastStaffingChanged(ctx, "sick_cascade_clear")
	s.getLogger().Info("sick cascade cleared",
		"staff_id", report.subjectStaffID,
		"absence_id", report.absenceID,
	)
	return nil
}

// ReconcileSickRange applies only the days removed from or added to an edited
// full-day sick report. The staff lock is acquired before either side reads
// shifts, and every affected future day lock is acquired in ascending order
// before block writes, so the two directions cannot deadlock each other.
func (s *sickCascade) ReconcileSickRange(ctx context.Context, beforeInput, afterInput workforce.SickCascadeInput) error {
	before, err := parseSickReport(beforeInput)
	if err != nil {
		return err
	}
	after, err := parseSickReport(afterInput)
	if err != nil {
		return err
	}
	if before.subjectStaffID != after.subjectStaffID || before.absenceID != after.absenceID {
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

func (s *sickCascade) broadcastStaffingChanged(ctx context.Context, source string) {
	broadcastStaffingChanged(ctx, s.broadcaster, s.getLogger(), source)
}

// lockSickReversalStaffWrites discovers every replacement owner attached to a
// shift stamped by this sick report, then acquires the complete staff lock set
// in the same global order used by ApplyCancellation. Taking the subject lock
// first and discovering a lower-ID replacement later creates the inverse lock
// order and can deadlock against a concurrent replacement edit.
func (s *sickCascade) lockSickReversalStaffWrites(ctx context.Context, report sickReport) error {
	shifts, err := s.stampedShifts(ctx, report.absenceID)
	if err != nil {
		return fmt.Errorf("sick clear: discover stamped shifts: %w", err)
	}
	staffIDs := []int64{report.subjectStaffID}
	originShiftIDs := staffShiftIDs(shifts)
	var covers []workforce.StaffShift
	if len(originShiftIDs) > 0 {
		// A non-nil empty OriginShiftIDs matches nothing, so the unfiltered
		// listing is never reached through this path.
		covers, err = s.shifts.ListStaffShifts(ctx, workforce.StaffShiftFilter{
			OriginShiftIDs: originShiftIDs,
			Order:          []workforce.StaffShiftOrder{{Field: workforce.StaffShiftOrderID}},
		})
	}
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

// stampedShifts lists the shifts this sick report owns, oldest row first.
func (s *sickCascade) stampedShifts(ctx context.Context, absenceID int64) ([]workforce.StaffShift, error) {
	return s.shifts.ListStaffShifts(ctx, workforce.StaffShiftFilter{
		SickAbsenceID: &absenceID,
		Order:         []workforce.StaffShiftOrder{{Field: workforce.StaffShiftOrderID}},
	})
}

func staffShiftIDs(shifts []workforce.StaffShift) []int64 {
	ids := make([]int64, 0, len(shifts))
	for _, shift := range shifts {
		ids = append(ids, shift.ID)
	}
	return ids
}

func (s *sickCascade) reconcileRemovedSickDays(ctx context.Context, before sickReport, removed map[timezone.Date]bool, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	if len(removed) == 0 {
		return nil
	}
	if err := s.reactivateStampedShifts(ctx, before, removed); err != nil {
		return err
	}
	return s.clearStampedBlocks(ctx, before, removed, activeTouched)
}

func (s *sickCascade) reconcileAddedSickDays(ctx context.Context, after sickReport, added map[timezone.Date]bool, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	if len(added) == 0 {
		return nil
	}
	days := sortedDateSet(added)
	if err := s.cancelShiftsForSickDays(ctx, after, days); err != nil {
		return err
	}
	return s.markBlocksForSickDays(ctx, after, days, activeTouched)
}

func (s *sickCascade) acquireCascadeDayLocks(ctx context.Context, days map[timezone.Date]bool) error {
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

func (s *sickCascade) reactivateStampedShifts(ctx context.Context, report sickReport, onlyDays map[timezone.Date]bool) error {
	shifts, err := s.stampedShifts(ctx, report.absenceID)
	if err != nil {
		return fmt.Errorf("sick clear: load stamped shifts: %w", err)
	}
	today := s.todayDate()
	for _, shift := range shifts {
		date, err := shiftDate(shift)
		if err != nil {
			return err
		}
		if onlyDays != nil && !onlyDays[date] {
			continue
		}
		if date.Before(today) {
			// Keep the historical cancellation intact, but release the deleted
			// or edited report's provenance stamp just as past block rows do.
			if err := s.releaseStampedShift(ctx, shift, 0); err != nil {
				return err
			}
			continue
		}
		if err := s.reactivateStampedShift(ctx, report, shift); err != nil {
			return err
		}
	}
	return nil
}

func (s *sickCascade) reactivateStampedShift(ctx context.Context, report sickReport, shift workforce.StaffShift) error {
	covers, err := s.shifts.ListStaffShifts(ctx, workforce.StaffShiftFilter{
		OriginShiftID: shift.ID,
		Order:         []workforce.StaffShiftOrder{{Field: workforce.StaffShiftOrderStartTime}},
	})
	if err != nil {
		return fmt.Errorf("sick clear: load covers of shift %d: %w", shift.ID, err)
	}
	if !shift.Cancelled || len(covers) > 0 {
		return s.releaseStampedShift(ctx, shift, len(covers))
	}
	// Reactivation clears sick_absence_id and change_reason via the ordinary
	// update path and re-checks overlap against the freed window.
	if _, err := s.shifts.ApplyCancellation(ctx, workforce.CancelStaffShift{
		ShiftID: shift.ID, Cancelled: false, ActorStaffID: report.actorStaffID,
	}); err != nil {
		return fmt.Errorf("sick clear: reactivate shift %d: %w", shift.ID, err)
	}
	return nil
}

func (s *sickCascade) releaseStampedShift(ctx context.Context, shift workforce.StaffShift, replacementCount int) error {
	if _, err := s.shifts.SetStaffShiftSickAbsence(ctx, shift.ID, nil); err != nil {
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

func (s *sickCascade) clearStampedBlocks(ctx context.Context, report sickReport, onlyDays map[timezone.Date]bool, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
	rows, err := s.loadStampedBlockRows(ctx, report.absenceID)
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
		if err := s.clearStampedBlocksForDay(ctx, report, day, byDay[day], activeTouched); err != nil {
			return err
		}
	}
	return nil
}

type stampedSickBlockRow struct {
	row      *scheduleModel.InstanceStaff
	instance *scheduleModel.ActivityInstance
}

func (s *sickCascade) loadStampedBlockRows(ctx context.Context, absenceID int64) ([]*scheduleModel.InstanceStaff, error) {
	listOptions := modelBase.NewQueryOptions()
	listOptions.Filter.Equal("sick_absence_id", absenceID)
	rows, err := legacyList[*scheduleModel.InstanceStaff](ctx, s.instanceStaffRepo, listOptions)
	if err != nil {
		return nil, fmt.Errorf("sick clear: load stamped rows: %w", err)
	}
	return rows, nil
}

func (s *sickCascade) classifyStampedBlockRows(ctx context.Context, rows []*scheduleModel.InstanceStaff, onlyDays map[timezone.Date]bool) (map[timezone.Date][]stampedSickBlockRow, []*scheduleModel.InstanceStaff, error) {
	today := s.todayDate()
	byDay := make(map[timezone.Date][]stampedSickBlockRow)
	var releaseOnly []*scheduleModel.InstanceStaff
	instancesByID, err := s.timetableData.GetActivityInstancesByID(ctx, instanceStaffInstanceIDs(rows))
	if err != nil {
		return nil, nil, fmt.Errorf("sick clear: load stamped instances: %w", err)
	}
	if missingID := missingActivityInstanceID(rows, instancesByID); missingID > 0 {
		return nil, nil, fmt.Errorf("sick clear: load instance %d: %w", missingID, modelBase.ErrNotFound)
	}
	for _, row := range rows {
		instance := instancesByID[row.InstanceID]
		if instance != nil && onlyDays != nil && !onlyDays[timezone.Date(instance.Date)] {
			continue
		}
		// Past days stay as recorded history; rows a manual edit already
		// restored, and terminal instances, only shed their stamp.
		if instance == nil || !row.IsAbsent || instance.Date.Before(today) || !isPlannableInstance(instance) {
			releaseOnly = append(releaseOnly, row)
			continue
		}
		date := timezone.Date(instance.Date)
		byDay[date] = append(byDay[date], stampedSickBlockRow{row: row, instance: instance})
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

func missingActivityInstanceID(rows []*scheduleModel.InstanceStaff, instances map[int64]*scheduleModel.ActivityInstance) int64 {
	for _, row := range rows {
		if instances[row.InstanceID] == nil {
			return row.InstanceID
		}
	}
	return 0
}

func (s *sickCascade) releaseSickBlockStamps(ctx context.Context, rows []*scheduleModel.InstanceStaff) error {
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

func (s *sickCascade) clearStampedBlocksForDay(ctx context.Context, report sickReport, day timezone.Date, entries []stampedSickBlockRow, activeTouched map[int64]*scheduleModel.ActivityInstance) error {
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
		if err := s.instances.ClearSickAbsence(ctx, entry.row, entry.instance, report.absenceID, report.actorAccountID, activeTouched); err != nil {
			return fmt.Errorf("sick clear: clear block %d: %w", entry.row.InstanceID, err)
		}
		if err := s.instances.ClearUnderstaffedAckIfStaffed(ctx, entry.row.InstanceID, report.actorAccountID); err != nil {
			return fmt.Errorf("sick clear: reconcile understaffed acknowledgement for block %d: %w", entry.row.InstanceID, err)
		}
	}
	return nil
}

// ReassignSickStamps re-points every stamp from one absence id to another
// (overlap merge: the secondary absence row is deleted, the surviving primary
// takes over its plan effects, #1843).
func (s *sickCascade) ReassignSickStamps(ctx context.Context, fromAbsenceID, toAbsenceID int64) error {
	shifts, err := s.stampedShifts(ctx, fromAbsenceID)
	if err != nil {
		return fmt.Errorf("sick reassign: load stamped shifts: %w", err)
	}
	for _, shift := range shifts {
		if _, err := s.shifts.SetStaffShiftSickAbsence(ctx, shift.ID, &toAbsenceID); err != nil {
			return fmt.Errorf("sick reassign: shift %d: %w", shift.ID, err)
		}
	}
	listOptions := modelBase.NewQueryOptions()
	listOptions.Filter.Equal("sick_absence_id", fromAbsenceID)
	rows, err := legacyList[*scheduleModel.InstanceStaff](ctx, s.instanceStaffRepo, listOptions)
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

func (s *sickCascade) instanceHasActiveSubstitute(ctx context.Context, instanceID int64) (bool, error) {
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

// sickReport is the parsed form of one SickCascadeInput. The public contract
// carries calendar days as DateLayout strings, so an entry point parses the
// range once and every comparison below works on timezone.Date.
type sickReport struct {
	subjectStaffID int64
	absenceID      int64
	actorStaffID   int64
	actorAccountID *int64
	dateStart      timezone.Date
	dateEnd        timezone.Date
	skipStartDay   bool
	skipEndDay     bool
}

func parseSickReport(in workforce.SickCascadeInput) (sickReport, error) {
	start, err := timezone.ParseDate(in.DateStart)
	if err != nil {
		return sickReport{}, fmt.Errorf("sick cascade: invalid date_start %q: %w", in.DateStart, err)
	}
	end, err := timezone.ParseDate(in.DateEnd)
	if err != nil {
		return sickReport{}, fmt.Errorf("sick cascade: invalid date_end %q: %w", in.DateEnd, err)
	}
	report := stampScope(in)
	report.dateStart, report.dateEnd = start, end
	return report, nil
}

// stampScope is the date-free part of one input, for the reversal that reads
// no day at all.
func stampScope(in workforce.SickCascadeInput) sickReport {
	return sickReport{
		subjectStaffID: in.SubjectStaffID,
		absenceID:      in.AbsenceID,
		actorStaffID:   in.ActorStaffID,
		actorAccountID: in.ActorAccountID,
		skipStartDay:   in.SkipStartDay,
		skipEndDay:     in.SkipEndDay,
	}
}

// shiftDate reads a stored shift's calendar day. A row the database returns is
// always a valid DateLayout value; a malformed one is a data error the
// fail-closed cascade must surface instead of skipping the row.
func shiftDate(shift workforce.StaffShift) (timezone.Date, error) {
	date, err := timezone.ParseDate(shift.Date)
	if err != nil {
		return "", fmt.Errorf("sick cascade: shift %d has an invalid date %q: %w", shift.ID, shift.Date, err)
	}
	return date, nil
}

// sickCascadeDays expands the absence range into the full sick days, skipping
// half boundary days — a half sick day never cascades (#1843 product rule).
func sickCascadeDays(report sickReport) []timezone.Date {
	var days []timezone.Date
	for d := report.dateStart; !d.After(report.dateEnd); d = d.AddDays(1) {
		if report.skipStartDay && d == report.dateStart {
			continue
		}
		if report.skipEndDay && d == report.dateEnd {
			continue
		}
		days = append(days, d)
	}
	return days
}

func sickCascadeDayDifference(before, after sickReport) (removed, added map[timezone.Date]bool) {
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
