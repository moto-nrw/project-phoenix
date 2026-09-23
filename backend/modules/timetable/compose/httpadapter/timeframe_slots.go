package httpadapter

import (
	"context"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The Timetable owner keeps clock values as timezone-free HH:MM:SS strings
// at its boundary; the schedules API renders them as instants on the zero
// date, exactly as the retained service used to hand them over.
const clockLayout = "15:04:05"

// timeframeSlot is one timeframe, or one free gap between timeframes, as an
// instant pair for the wire.
type timeframeSlot struct {
	ID          int64
	StartTime   time.Time
	EndTime     *time.Time
	IsActive    bool
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func timeframeToSlot(value timetableModule.Timeframe) (timeframeSlot, error) {
	start, err := parseClock(value.StartTime)
	if err != nil {
		return timeframeSlot{}, err
	}
	slot := timeframeSlot{
		ID: value.ID, StartTime: start, IsActive: value.IsActive, Description: value.Description,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
	if value.EndTime != nil {
		end, parseErr := parseClock(*value.EndTime)
		if parseErr != nil {
			return timeframeSlot{}, parseErr
		}
		slot.EndTime = &end
	}
	return slot, nil
}

func timeframesToSlots(values []timetableModule.Timeframe) ([]timeframeSlot, error) {
	slots := make([]timeframeSlot, 0, len(values))
	for _, value := range values {
		slot, err := timeframeToSlot(value)
		if err != nil {
			return nil, err
		}
		slots = append(slots, slot)
	}
	return slots, nil
}

// clockOf renders an instant's wall clock the way the owner stores and
// filters on it; the API never writes a fractional second.
func clockOf(value time.Time) string { return value.Format(clockLayout) }

// parseClock reads the owner's clock string back; a stored row may still
// carry a fractional second, which the wire keeps.
func parseClock(value string) (time.Time, error) {
	clock, err := time.Parse(clockLayout, value)
	if err == nil {
		return clock, nil
	}
	return time.Parse(clockLayout+".999999999", value)
}

func timeframeInput(start time.Time, end *time.Time, isActive bool, description string) timetableModule.TimeframeInput {
	input := timetableModule.TimeframeInput{StartTime: clockOf(start), IsActive: isActive, Description: description}
	if end != nil {
		clock := clockOf(*end)
		input.EndTime = &clock
	}
	return input
}

// overlappingTimeframes lists the timeframes touching [start, end].
func overlappingTimeframes(ctx context.Context, timeframes timetableModule.TimeframeQuery, start, end time.Time) ([]timeframeSlot, error) {
	from, to := clockOf(start), clockOf(end)
	values, err := timeframes.ListTimeframes(ctx, timetableModule.TimeframeFilter{OverlapsStart: &from, OverlapsEnd: &to})
	if err != nil {
		return nil, err
	}
	return timeframesToSlots(values)
}

// checkTimeframeConflict reports whether any timeframe overlaps the range.
func checkTimeframeConflict(ctx context.Context, timeframes timetableModule.TimeframeQuery, start, end time.Time) (bool, []timeframeSlot, error) {
	if !end.IsZero() && start.After(end) {
		return false, nil, timetableModule.ErrInvalidTimeRange
	}
	conflicting, err := overlappingTimeframes(ctx, timeframes, start, end)
	if err != nil {
		return false, nil, err
	}
	return len(conflicting) > 0, conflicting, nil
}

// availableTimeframeSlots finds the gaps of at least duration between the
// timeframes overlapping [start, end]. Every clock is normalized the way the
// retained schedule service did before comparing.
func availableTimeframeSlots(ctx context.Context, timeframes timetableModule.TimeframeQuery, start, end time.Time, duration time.Duration) ([]timeframeSlot, error) {
	if start.After(end) {
		return nil, timetableModule.ErrInvalidRecurrenceRange
	}
	if duration <= 0 {
		return nil, timetableModule.ErrInvalidDuration
	}
	start = timezone.NormalizeWallClock(start)
	end = timezone.NormalizeWallClock(end)

	existing, err := overlappingTimeframes(ctx, timeframes, start, end)
	if err != nil {
		return nil, err
	}
	sort.Slice(existing, func(i, j int) bool {
		return timezone.NormalizeWallClock(existing[i].StartTime).Before(timezone.NormalizeWallClock(existing[j].StartTime))
	})

	var available []timeframeSlot
	current := start
	for _, tf := range existing {
		tfStart := timezone.NormalizeWallClock(tf.StartTime)
		if current.Before(tfStart) {
			gapEnd := tfStart
			if gapEnd.Sub(current) >= duration {
				available = append(available, timeframeSlot{StartTime: current, EndTime: &gapEnd, IsActive: true})
			}
		}
		if tf.EndTime == nil {
			// Open-ended timeframe, no more available slots.
			return available, nil
		}
		current = timezone.NormalizeWallClock(*tf.EndTime)
	}
	if current.Before(end) && end.Sub(current) >= duration {
		gapEnd := end
		available = append(available, timeframeSlot{StartTime: current, EndTime: &gapEnd, IsActive: true})
	}
	return available, nil
}
