package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// UpdateSeriesInput carries a whole-series edit ("Alle Termine der Serie",
// #2028). Unlike SplitSeriesInput it rewrites the rule in place instead of
// creating a successor: there is no effective date, the segment keeps its
// identity, and every regenerable future occurrence is rebuilt from the new
// values. Optional fields (nil) keep the stored value, so a client editing
// only the window never has to re-send the rhythm.
type UpdateSeriesInput struct {
	SeriesID     int64
	Weekdays     []int16 // nil = keep stored weekdays
	StartTime    time.Time
	EndTime      time.Time
	BreakMinutes int
	ShiftTypeID  *int64
	// ShiftTypeIDSet distinguishes "clear the type" (true, nil value) from
	// "keep the stored type" (false) — same tri-state as the split payload.
	ShiftTypeIDSet bool
	Notes          *string // nil = keep stored notes
	WeekPattern    *int    // nil = keep stored pattern
	ValidUntil     *timezone.Date
	// ValidUntilSet distinguishes "run until the period ends" (true, nil
	// value) from "keep the stored end" (false).
	ValidUntilSet bool
	ActorStaffID  int64
}

// SeriesDeviationKind classifies one individually adjusted occurrence of a
// series. The whole-series edit treats the kinds differently, so the planner
// can be told what survives before anything is written.
type SeriesDeviationKind string

const (
	// SeriesDeviationTimeEdit is an occurrence whose window/type was changed
	// for a single day ("Nur diese Woche") or that was moved. A whole-series
	// edit regenerates it from the rule, so the deviation is lost.
	SeriesDeviationTimeEdit SeriesDeviationKind = "time_edit"
	// SeriesDeviationCancelled is an occurrence marked as not taking place.
	// It survives a whole-series edit: dropping it would silently resurrect
	// the shift and strand the replacement shifts covering its gap (#1841).
	SeriesDeviationCancelled SeriesDeviationKind = "cancelled"
	// SeriesDeviationRemoved is an occurrence deleted for a single day. The
	// stored exception keeps it from being regenerated.
	SeriesDeviationRemoved SeriesDeviationKind = "removed"
)

// SeriesDeviation is one individually adjusted occurrence, identified by its
// recurrence slot (not the current date — a moved row still belongs to the
// slot it was generated for).
type SeriesDeviation struct {
	Date      timezone.Date
	Kind      SeriesDeviationKind
	StartTime time.Time
	EndTime   time.Time
	// Moved reports that the row no longer sits on its recurrence slot
	// (different day or staff member).
	Moved bool
}

// SeriesDeviations splits the upcoming deviations of one series into the ones
// a whole-series edit rebuilds and the ones it leaves untouched.
type SeriesDeviations struct {
	// From is the first date a whole-series edit would touch — today and
	// earlier are never re-planned.
	From        timezone.Date
	Overwritten []SeriesDeviation
	Preserved   []SeriesDeviation
}

// rebuildFrom is the first date a whole-series edit may touch: never today or
// earlier (the materialization rule for every series operation), and never
// before the segment starts.
func rebuildFrom(series *scheduleModels.StaffShiftSeries) timezone.Date {
	from := timezone.TodayDate().AddDays(1)
	if series.ValidFrom.After(from) {
		from = series.ValidFrom
	}
	return from
}

func (s *staffShiftSeriesService) GetSeries(ctx context.Context, seriesID int64) (*scheduleModels.StaffShiftSeries, error) {
	return s.loadSeries(ctx, seriesID)
}

func (s *staffShiftSeriesService) SeriesDeviations(ctx context.Context, seriesID int64) (*SeriesDeviations, error) {
	series, err := s.loadSeries(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	from := rebuildFrom(series)
	result := &SeriesDeviations{From: from}

	detached, err := s.shiftRepo.FindDetachedBySeriesFrom(ctx, series.ID, from)
	if err != nil {
		return nil, err
	}
	for _, shift := range detached {
		slot := seriesOccurrenceDate(shift)
		deviation := SeriesDeviation{
			Date:      slot,
			StartTime: shift.StartTime,
			EndTime:   shift.EndTime,
			Moved:     shift.Date != slot || shift.StaffID != series.StaffID,
		}
		if shift.Cancelled {
			deviation.Kind = SeriesDeviationCancelled
			result.Preserved = append(result.Preserved, deviation)
			continue
		}
		deviation.Kind = SeriesDeviationTimeEdit
		result.Overwritten = append(result.Overwritten, deviation)
	}

	exceptionDates, err := s.exceptionRepo.FindDatesBySeriesID(ctx, series.ID)
	if err != nil {
		return nil, err
	}
	for _, date := range exceptionDates {
		if date.Before(from) {
			continue
		}
		result.Preserved = append(result.Preserved, SeriesDeviation{Date: date, Kind: SeriesDeviationRemoved})
	}
	return result, nil
}

// UpdateSeries rewrites the rule in place and rebuilds its future
// occurrences. Occurrences on or before today are never touched, cancelled
// days keep their cancellation (and with it the replacement shifts covering
// them), and deliberately removed days stay removed — everything else is
// regenerated from the new rule.
func (s *staffShiftSeriesService) UpdateSeries(ctx context.Context, input UpdateSeriesInput) (*SeriesResult, error) {
	series, err := s.loadSeries(ctx, input.SeriesID)
	if err != nil {
		return nil, err
	}
	previousShiftTypeID := series.ShiftTypeID

	if input.Weekdays != nil {
		series.Weekdays = input.Weekdays
	}
	series.StartTime = input.StartTime
	series.EndTime = input.EndTime
	series.BreakMinutes = input.BreakMinutes
	if input.ShiftTypeIDSet {
		series.ShiftTypeID = input.ShiftTypeID
	}
	if input.Notes != nil {
		series.Notes = *input.Notes
	}
	if input.WeekPattern != nil {
		series.WeekPattern = *input.WeekPattern
	}
	if input.ValidUntilSet {
		series.ValidUntil = input.ValidUntil
	}
	if input.ActorStaffID > 0 {
		actor := input.ActorStaffID
		series.UpdatedBy = &actor
	}

	if err := series.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSeriesInvalid, err.Error())
	}
	if !sameShiftTypeID(series.ShiftTypeID, previousShiftTypeID) {
		if err := s.ensureShiftTypeActive(ctx, series.ShiftTypeID); err != nil {
			return nil, err
		}
	}
	period, err := s.loadPeriodForSeries(ctx, series)
	if err != nil {
		return nil, err
	}

	if err := s.lockShiftWrites(ctx, series.StaffID); err != nil {
		return nil, err
	}
	if err := s.seriesRepo.Update(ctx, series); err != nil {
		return nil, err
	}

	from := rebuildFrom(series)
	deleted, err := s.shiftRepo.DeleteNonDetachedBySeriesFrom(ctx, series.ID, from)
	if err != nil {
		return nil, err
	}
	overwritten, err := s.dropRegenerableDeviations(ctx, series.ID, from)
	if err != nil {
		return nil, err
	}
	deleted += overwritten

	created, skipped, err := s.materializeSeries(ctx, series, period)
	if err != nil {
		return nil, err
	}
	s.getLogger().Info("staff shift series updated",
		"series_id", series.ID,
		"staff_id", series.StaffID,
		"from", from.String(),
		"deleted", deleted,
		"overwritten_deviations", overwritten,
		"created", created,
		"skipped", len(skipped),
	)
	s.broadcastTimeTrackingChanged(ctx)
	return &SeriesResult{Series: series, Created: created, Deleted: deleted, SkippedDates: skipped}, nil
}

// dropRegenerableDeviations removes the detached rows a whole-series edit
// rebuilds. Cancelled rows are kept: they carry the "fällt aus" state the
// replacement shifts of other people hang off, and materializeSeries treats
// their slot as owned, so the day stays cancelled instead of reappearing as a
// regular shift.
func (s *staffShiftSeriesService) dropRegenerableDeviations(ctx context.Context, seriesID int64, from timezone.Date) (int64, error) {
	detached, err := s.shiftRepo.FindDetachedBySeriesFrom(ctx, seriesID, from)
	if err != nil {
		return 0, err
	}
	var removed int64
	for _, shift := range detached {
		if shift.Cancelled {
			continue
		}
		if err := s.shiftRepo.Delete(ctx, shift.ID); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
