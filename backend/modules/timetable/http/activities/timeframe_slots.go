package httpadapter

import (
	"time"

	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The Timetable owner keeps clock values as timezone-free HH:MM:SS strings
// at its boundary; the schedules API renders them as instants on the zero
// date, exactly as the retained service used to hand them over.
const clockLayout = "15:04:05"

// timeframeSlot is one timeframe as an instant pair for the wire.
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

// parseClock reads the owner's clock string back; a stored row may still
// carry a fractional second, which the wire keeps.
func parseClock(value string) (time.Time, error) {
	clock, err := time.Parse(clockLayout, value)
	if err == nil {
		return clock, nil
	}
	return time.Parse(clockLayout+".999999999", value)
}
