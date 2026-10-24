package planning

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *staffShiftSeriesService) SplitSeries(ctx context.Context, input SplitSeriesInput) (*SeriesResult, error) {
	old, err := s.loadSeries(ctx, input.SeriesID)
	if err != nil {
		return nil, err
	}

	today := s.todayDate()
	updateToday := input.EffectiveDate == today
	effective, validUntil, err := splitSeriesBounds(old, input, today)
	if err != nil {
		return nil, err
	}
	rootID := old.RootID()
	successor, err := s.buildSplitSuccessor(ctx, old, input, rootID, effective, validUntil)
	if err != nil {
		return nil, err
	}
	period, err := s.loadPeriodForSeries(ctx, successor)
	if err != nil {
		return nil, err
	}
	if err := s.lockShiftWrites(ctx, old.StaffID); err != nil {
		return nil, err
	}
	if err := s.boundSuccessorByLineage(ctx, successor, rootID, old.ID, effective); err != nil {
		return nil, err
	}
	if !hasFutureSeriesOccurrence(successor, period, today) {
		return nil, withReason(fmt.Errorf(
			"%w: no occurrences left to change for the selected weekdays and week pattern",
			ErrSeriesInvalid,
		), workforce.ErrShiftSeriesNoOccurrences)
	}
	repointFrom := effective
	if updateToday {
		if err := s.retainTodayOccurrence(ctx, old, input, successor); err != nil {
			return nil, err
		}
		repointFrom = input.EffectiveDate
	}
	deleted, err := s.replaceSeriesSegment(ctx, old.ID, successor, effective, repointFrom)
	if err != nil {
		return nil, err
	}
	run, err := s.materializeSeries(ctx, successor, period)
	if err != nil {
		return nil, err
	}
	s.logSeriesSplit(old, successor, effective, deleted, run)
	s.broadcastTimeTrackingChanged(ctx)
	return &SeriesResult{
		Series: successor, OldSeriesID: old.ID, Created: run.created, Deleted: deleted,
		SkippedDates: run.skipped, SkippedNonWorkingDays: run.nonWorkingSkipped,
	}, nil
}

// splitSeriesBounds clamps the split's effective date to tomorrow and the
// predecessor's start, and resolves the successor's end.
//
// The successor is bounded by the EDITED end, not the predecessor's: an
// editor that extends "Gültig bis" is deliberately re-opening a series whose
// stored end already passed, and that is a legitimate edit. Checking the old
// bound here made every series whose last day had arrived permanently
// uneditable, because the effective date is clamped to tomorrow (#2028).
func splitSeriesBounds(old *StaffShiftSeries, input SplitSeriesInput, today calendar.Date) (calendar.Date, *calendar.Date, error) {
	effective := input.EffectiveDate
	tomorrow := today.AddDays(1)
	if tomorrow.After(effective) {
		effective = tomorrow
	}
	if effective.Before(old.ValidFrom) {
		effective = old.ValidFrom
	}
	validUntil := old.ValidUntil
	if input.ValidUntilSet {
		validUntil = nil
		if input.ValidUntil != nil {
			converted := *input.ValidUntil
			validUntil = &converted
		}
	}
	if validUntil != nil && !effective.Before(*validUntil) {
		return effective, nil, withReason(fmt.Errorf(
			"%w: series ends before %s, no occurrences left to change",
			ErrSeriesInvalid, effective.String(),
		), workforce.ErrShiftSeriesNoOccurrences)
	}
	return effective, validUntil, nil
}

// buildSplitSuccessor builds and validates the successor segment; a newly
// assigned shift type must be active.
func (s *staffShiftSeriesService) buildSplitSuccessor(
	ctx context.Context,
	old *StaffShiftSeries,
	input SplitSeriesInput,
	rootID int64,
	effective calendar.Date,
	validUntil *calendar.Date,
) (*StaffShiftSeries, error) {
	successor := successorSeries(old, input, rootID, effective, validUntil)
	successor.TenantID = old.TenantID
	if err := successor.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSeriesInvalid, err.Error())
	}
	if !sameShiftTypeID(successor.ShiftTypeID, old.ShiftTypeID) {
		if err := s.ensureShiftTypeActive(ctx, successor.ShiftTypeID); err != nil {
			return nil, err
		}
	}
	return successor, nil
}

// boundSuccessorByLineage ends the successor before a later segment of the
// same lineage. A predecessor can be edited before a later segment begins.
// Keep the new segment strictly before that successor, but never reopen a
// segment that a current successor has already superseded.
func (s *staffShiftSeriesService) boundSuccessorByLineage(ctx context.Context, successor *StaffShiftSeries, rootID, oldID int64, effective calendar.Date) error {
	nextValue, err := s.seriesRepo.FindOverlappingSeriesInLineage(ctx, rootID, oldID, effective.String())
	if err != nil && !errors.Is(err, workforce.ErrShiftSeriesNotFound) {
		return shiftReadError("find overlapping staff shift series in lineage", err, nil)
	}
	var next *StaffShiftSeries
	if err == nil {
		next = seriesToRow(nextValue)
	}
	if next != nil && !effective.Before(next.ValidFrom) {
		return fmt.Errorf("%w: series segment has already been superseded", ErrSeriesInvalid)
	}
	if next != nil && (successor.ValidUntil == nil || next.ValidFrom.Before(*successor.ValidUntil)) {
		until := next.ValidFrom
		successor.ValidUntil = &until
	}
	return nil
}

// retainTodayOccurrence updates today's opened occurrence in place and marks it
// as the successor's retained occurrence when that update happened.
func (s *staffShiftSeriesService) retainTodayOccurrence(ctx context.Context, old *StaffShiftSeries, input SplitSeriesInput, successor *StaffShiftSeries) error {
	updateRetainedOccurrence := old.RetainedOccurrenceShiftID != nil &&
		*old.RetainedOccurrenceShiftID == input.OccurrenceShiftID
	updated, err := s.updateTodayOccurrence(ctx, input, updateRetainedOccurrence)
	if err != nil {
		return err
	}
	if updated {
		retainedID := input.OccurrenceShiftID
		successor.RetainedOccurrenceShiftID = &retainedID
	}
	return nil
}

// replaceSeriesSegment caps the predecessor, removes its regenerable rows from
// effective, stores the successor and hands the deviations over to it. It
// returns the number of deleted rows.
//
// Deviation preservation (#1890 counterpart): detached rows and removed
// occurrences from the effective date onward belong to the successor now.
// A retained today occurrence was just detached in place, so it must join
// the successor too (repointFrom is today then); otherwise reopening it would
// submit the capped segment ID and a second permanent edit would be rejected
// as superseded.
func (s *staffShiftSeriesService) replaceSeriesSegment(ctx context.Context, oldID int64, successor *StaffShiftSeries, effective, repointFrom calendar.Date) (int64, error) {
	if err := shiftWriteError("cap staff shift series valid_until", s.seriesRepo.CapStaffShiftSeries(ctx, oldID, effective.String())); err != nil {
		return 0, err
	}
	deleted, err := s.shiftRepo.DeleteRegenerableSeriesShifts(ctx, oldID, effective.String())
	err = shiftWriteError("delete non-detached staff shifts by series", err)
	if err != nil {
		return 0, err
	}
	written, err := s.seriesRepo.CreateStaffShiftSeries(ctx, seriesRowToCapability(successor))
	if err != nil {
		err = shiftWriteError("create", err)

		return 0, err
	}
	applySeriesToRow(successor, written)
	if _, err := s.shiftRepo.RepointDetachedSeriesShifts(ctx, oldID, successor.ID, repointFrom.String()); err != nil {
		err = shiftWriteError("repoint detached staff shifts to successor series", err)
		return 0, err
	}
	if _, err := s.exceptionRepo.RepointSeriesExceptions(ctx, oldID, successor.ID, effective.String()); err != nil {
		err = shiftWriteError("repoint staff shift series exceptions", err)
		return 0, err
	}
	return deleted, nil
}

func (s *staffShiftSeriesService) logSeriesSplit(old, successor *StaffShiftSeries, effective calendar.Date, deleted int64, run seriesMaterialization) {
	s.getLogger().Info("staff shift series split",
		"series_id", old.ID,
		"successor_id", successor.ID,
		"staff_id", old.StaffID,
		"effective", effective.String(),
		"deleted", deleted,
		"created", run.created,
		"skipped", len(run.skipped),
		"non_working_skipped", run.nonWorkingSkipped,
	)
}

// successorSeries builds the segment a split creates. Fields the edit left
// out (nil) inherit the predecessor's value; staff, period and lineage stay
// with the series.
func successorSeries(old *StaffShiftSeries, input SplitSeriesInput, rootID int64, effective calendar.Date, validUntil *calendar.Date) *StaffShiftSeries {
	successor := &StaffShiftSeries{
		StaffID:             old.StaffID,
		Weekdays:            old.Weekdays,
		StartTime:           input.StartTime,
		EndTime:             input.EndTime,
		BreakMinutes:        input.BreakMinutes,
		ShiftTypeID:         old.ShiftTypeID,
		Notes:               old.Notes,
		CalendarPeriodID:    old.CalendarPeriodID,
		WeekPattern:         old.WeekPattern,
		IncludeSchoolBreaks: old.IncludeSchoolBreaks,
		ValidFrom:           effective,
		ValidUntil:          validUntil,
		SeriesRootID:        &rootID,
		CreatedBy:           input.ActorStaffID,
	}
	if input.Weekdays != nil {
		successor.Weekdays = input.Weekdays
	}
	if input.WeekPattern != nil {
		successor.WeekPattern = *input.WeekPattern
	}
	if input.ShiftTypeIDSet {
		successor.ShiftTypeID = input.ShiftTypeID
	}
	if input.Notes != nil {
		successor.Notes = *input.Notes
	}
	if input.IncludeSchoolBreaks != nil {
		successor.IncludeSchoolBreaks = *input.IncludeSchoolBreaks
	}
	return successor
}
