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
	"fmt"
	"log/slog"
	"sort"

	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/active"
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
	logger            *slog.Logger
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
	logger *slog.Logger,
) active.ShiftPlanSyncer {
	return &shiftPlanSyncService{
		shifts:            shifts,
		instances:         instances,
		timetableData:     timetableData,
		shiftRepo:         shiftRepo,
		instanceStaffRepo: instanceStaffRepo,
		logger:            logger,
	}
}

func (s *shiftPlanSyncService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
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

	if err := s.cancelShiftsForSickDays(ctx, in, days); err != nil {
		return err
	}
	if err := s.markBlocksForSickDays(ctx, in, days); err != nil {
		return err
	}
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
	for _, shift := range shifts {
		if !daySet[shift.Date] {
			continue // boundary half days never cascade
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

func (s *shiftPlanSyncService) markBlocksForSickDays(ctx context.Context, in active.SickCascadeInput, days []timezone.Date) error {
	// Past days are never touched: a completed/historical instance records
	// what actually happened (mirrors the deviations endpoints' past guard).
	today := timezone.TodayDate()
	reason := sickBlockAbsenceReason
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)
	for _, d := range days {
		if d.Before(today) {
			continue
		}
		if err := s.timetableData.AcquireSubstituteDayLock(ctx, d); err != nil {
			return fmt.Errorf("sick cascade: day lock %s: %w", d.String(), err)
		}
		rows, err := s.timetableData.GetInstanceStaffByStaffAndDate(ctx, in.SubjectStaffID, d)
		if err != nil {
			return fmt.Errorf("sick cascade: load assignments %s: %w", d.String(), err)
		}
		for _, row := range rows {
			if row.IsAbsent {
				continue // already absent (manual deviation or overlapping report)
			}
			instance, err := s.timetableData.GetActivityInstance(ctx, row.InstanceID)
			if err != nil {
				return fmt.Errorf("sick cascade: load instance %d: %w", row.InstanceID, err)
			}
			if instance == nil || !sickCascadePlannable(instance) {
				continue
			}
			if err := s.instances.ApplySickAbsence(ctx, row, instance, &reason, in.AbsenceID, in.ActorAccountID, activeTouched); err != nil {
				return fmt.Errorf("sick cascade: mark block %d: %w", row.InstanceID, err)
			}
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
	if err := s.reactivateStampedShifts(ctx, in); err != nil {
		return err
	}
	if err := s.clearStampedBlocks(ctx, in); err != nil {
		return err
	}
	s.getLogger().Info("sick cascade cleared",
		"staff_id", in.SubjectStaffID,
		"absence_id", in.AbsenceID,
	)
	return nil
}

func (s *shiftPlanSyncService) reactivateStampedShifts(ctx context.Context, in active.SickCascadeInput) error {
	shifts, err := s.shiftRepo.List(ctx, map[string]any{"sick_absence_id": in.AbsenceID})
	if err != nil {
		return fmt.Errorf("sick clear: load stamped shifts: %w", err)
	}
	for _, shift := range shifts {
		covers, err := s.shiftRepo.FindByOriginShiftID(ctx, shift.ID)
		if err != nil {
			return fmt.Errorf("sick clear: load covers of shift %d: %w", shift.ID, err)
		}
		if !shift.Cancelled || len(covers) > 0 {
			// Not cancelled: nothing to reactivate. Covered: reactivating
			// would delete the admin's replacements. Release the stamp only.
			shift.SickAbsenceID = nil
			if _, err := s.shiftRepo.UpdateColumns(ctx, shift, "sick_absence_id"); err != nil {
				return fmt.Errorf("sick clear: release shift %d: %w", shift.ID, err)
			}
			if len(covers) > 0 {
				s.getLogger().Info("sick clear kept a replaced shift cancelled",
					"shift_id", shift.ID,
					"staff_id", shift.StaffID,
					"replacements", len(covers),
				)
			}
			continue
		}
		// Reactivation clears sick_absence_id and change_reason via the
		// ordinary update path and re-checks overlap against the freed window.
		if _, err := s.shifts.ApplyCancellation(ctx, CancelShiftInput{
			ShiftID:      shift.ID,
			Cancelled:    false,
			ActorStaffID: in.ActorStaffID,
		}); err != nil {
			return fmt.Errorf("sick clear: reactivate shift %d: %w", shift.ID, err)
		}
	}
	return nil
}

func (s *shiftPlanSyncService) clearStampedBlocks(ctx context.Context, in active.SickCascadeInput) error {
	listOptions := modelBase.NewQueryOptions()
	listOptions.Filter.Equal("sick_absence_id", in.AbsenceID)
	rows, err := s.instanceStaffRepo.List(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("sick clear: load stamped rows: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	type stampedRow struct {
		row      *scheduleModel.InstanceStaff
		instance *scheduleModel.ActivityInstance
	}
	today := timezone.TodayDate()
	byDay := make(map[timezone.Date][]stampedRow)
	var releaseOnly []*scheduleModel.InstanceStaff
	for _, row := range rows {
		instance, err := s.timetableData.GetActivityInstance(ctx, row.InstanceID)
		if err != nil {
			return fmt.Errorf("sick clear: load instance %d: %w", row.InstanceID, err)
		}
		// Past days stay as recorded history; rows a manual edit already
		// restored, and terminal instances, only shed their stamp.
		if instance == nil || !row.IsAbsent || instance.Date.Before(today) || !sickCascadePlannable(instance) {
			releaseOnly = append(releaseOnly, row)
			continue
		}
		byDay[instance.Date] = append(byDay[instance.Date], stampedRow{row: row, instance: instance})
	}
	for _, row := range releaseOnly {
		row.SickAbsenceID = nil
		if _, err := s.instanceStaffRepo.UpdateColumns(ctx, row, "sick_absence_id"); err != nil {
			return fmt.Errorf("sick clear: release row %d: %w", row.ID, err)
		}
	}
	dates := make([]timezone.Date, 0, len(byDay))
	for d := range byDay {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)
	for _, d := range dates {
		if err := s.timetableData.AcquireSubstituteDayLock(ctx, d); err != nil {
			return fmt.Errorf("sick clear: day lock %s: %w", d.String(), err)
		}
		for _, entry := range byDay[d] {
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

// sickCascadePlannable mirrors the deviations endpoints' rule: only planned or
// currently active instances take staffing deviations.
func sickCascadePlannable(instance *scheduleModel.ActivityInstance) bool {
	return instance.Status == scheduleModel.InstanceStatusPlanned ||
		instance.Status == scheduleModel.InstanceStatusActive
}
