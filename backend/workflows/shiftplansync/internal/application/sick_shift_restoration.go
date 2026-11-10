package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

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
	if shift.SeriesID != nil {
		outside, err := s.outsideSeriesValidity(ctx, shift)
		if err != nil {
			return err
		}
		if outside {
			if err := s.shifts.DeleteStaffShift(ctx, shift.ID); err != nil {
				return fmt.Errorf("sick clear: discard obsolete series shift %d: %w", shift.ID, err)
			}
			return nil
		}
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

// A sick cancellation is detached and survives series end/split. The report
// must not restore an occurrence that its current segment no longer plans.
// Use the occurrence date so an intentional move keeps its original validity.
func (s *sickCascade) outsideSeriesValidity(ctx context.Context, shift workforce.StaffShift) (bool, error) {
	series, err := s.shifts.FindStaffShiftSeries(ctx, *shift.SeriesID)
	if err != nil {
		return false, fmt.Errorf("sick clear: load series for shift %d: %w", shift.ID, err)
	}
	occurrence := shift.SeriesOccurrenceDate
	if occurrence == "" {
		occurrence = shift.Date
	}
	date, err := calendar.ParseDate(occurrence)
	if err != nil {
		return false, fmt.Errorf("sick clear: invalid occurrence date for shift %d: %w", shift.ID, err)
	}
	from, err := calendar.ParseDate(series.ValidFrom)
	if err != nil {
		return false, fmt.Errorf("sick clear: invalid series start for shift %d: %w", shift.ID, err)
	}
	if series.ValidUntil == "" {
		return date.Before(from), nil
	}
	until, err := calendar.ParseDate(series.ValidUntil)
	if err != nil {
		return false, fmt.Errorf("sick clear: invalid series end for shift %d: %w", shift.ID, err)
	}
	return date.Before(from) || !date.Before(until), nil
}
