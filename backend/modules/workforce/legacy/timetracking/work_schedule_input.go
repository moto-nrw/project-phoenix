package timetracking

import (
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// This file turns a schedule-update request into schedule rows.

func buildScheduleEntries(reqEntries []ScheduleEntry, rotation int) ([]*WorkScheduleRow, []*WorkTimeTemplateEntry, error) {
	entries := make([]*WorkScheduleRow, 0, len(reqEntries))
	templateEntries := make([]*WorkTimeTemplateEntry, 0, len(reqEntries))
	seenSlots := make(map[string]struct{}, len(reqEntries))
	for _, e := range reqEntries {
		if e.TargetMinutes <= 0 {
			continue
		}
		if err := validateScheduleEntryRequest(e, rotation, seenSlots); err != nil {
			return nil, nil, err
		}
		startTime, err := parseScheduleStartTime(e.StartTime)
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, &WorkScheduleRow{
			WeekIndex:      e.WeekIndex,
			RotationLength: rotation,
			DayOfWeek:      e.DayOfWeek,
			TargetMinutes:  e.TargetMinutes,
			StartTime:      startTime,
		})
		templateEntries = append(templateEntries, &WorkTimeTemplateEntry{
			WeekIndex:     e.WeekIndex,
			DayOfWeek:     e.DayOfWeek,
			TargetMinutes: e.TargetMinutes,
			StartTime:     startTime,
		})
	}
	return entries, templateEntries, nil
}

func parseScheduleStartTime(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("15:04", *raw)
	if err != nil {
		return nil, scheduleValidationErrorf("start_time must be HH:MM")
	}
	wallClock := timezone.NormalizeWallClock(parsed)
	return &wallClock, nil
}

func validateScheduleEntryRequest(e ScheduleEntry, rotation int, seenSlots map[string]struct{}) error {
	if e.WeekIndex < 0 || e.WeekIndex >= rotation {
		return scheduleValidationErrorf("week_index %d outside rotation_length %d", e.WeekIndex, rotation)
	}
	if e.DayOfWeek < DayMonday || e.DayOfWeek > DaySunday {
		return scheduleValidationErrorf("day_of_week must be between 0 and 6")
	}
	if e.TargetMinutes > scheduleEntryMaxTargetMinutes {
		return scheduleValidationErrorf("target_minutes must be between 0 and %d", scheduleEntryMaxTargetMinutes)
	}
	slot := fmt.Sprintf("%d:%d", e.WeekIndex, e.DayOfWeek)
	if _, ok := seenSlots[slot]; ok {
		return scheduleValidationErrorf("duplicate schedule entry for week_index %d and day_of_week %d", e.WeekIndex, e.DayOfWeek)
	}
	seenSlots[slot] = struct{}{}
	return nil
}

// isRotationalSchedule reports whether the rows span more than one week, i.e.
// whether their parity depends on a rotation anchor at all. Single-week
// schedules have no parity, so they keep a NULL anchor.
func isRotationalSchedule(entries []*WorkScheduleRow) bool {
	for _, e := range entries {
		if e != nil && (e.RotationLength > 1 || e.WeekIndex > 0) {
			return true
		}
	}
	return false
}

// modelEntriesToScheduleRows converts work-time-model entries into schedule
// snapshot rows, dropping non-positive target minutes.
func modelEntriesToScheduleRows(modelEntries []*WorkTimeTemplateEntry, rotation int) []*WorkScheduleRow {
	rows := make([]*WorkScheduleRow, 0, len(modelEntries))
	for _, e := range modelEntries {
		if e.TargetMinutes <= 0 {
			continue
		}
		rows = append(rows, &WorkScheduleRow{
			WeekIndex:      e.WeekIndex,
			RotationLength: rotation,
			DayOfWeek:      e.DayOfWeek,
			TargetMinutes:  e.TargetMinutes,
			StartTime:      e.StartTime,
		})
	}
	return rows
}
