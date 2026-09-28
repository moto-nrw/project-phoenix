package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// WeeklyTargetOverrides re-prices the weeks a Sonderarbeitszeit touches
// (#3259) with the daily Soll the Monatskarte uses; weeks without one are
// absent from the result.
type WeeklyTargetOverrides interface {
	OverriddenWeeklyTargets(ctx context.Context, staffID int64, weekStarts []timezone.Date) (map[timezone.Date]int, error)
}

// withTargetOverrides replaces the weekly Soll of every week a
// Sonderarbeitszeit touches. A lookup failure shows no weekly Soll rather
// than one that ignores the override, like a holiday lookup failure does.
func (s *workSessionService) withTargetOverrides(ctx context.Context, staffID int64, sessions []*SessionResponse, targets map[summaryWeekKey]int) map[summaryWeekKey]int {
	if s.weeklyOverrides == nil || len(sessions) == 0 {
		return targets
	}
	corrected, err := s.weeklyOverrides.OverriddenWeeklyTargets(ctx, staffID, sessionWeekStarts(sessions))
	if err != nil {
		s.getLogger().Warn("failed to load target overrides for weekly summaries", "error", err.Error())
		return nil
	}
	if len(corrected) > 0 && targets == nil {
		targets = make(map[summaryWeekKey]int, len(corrected))
	}
	for weekStart, target := range corrected {
		targets[summaryKeyOf(weekStart)] = target
	}
	return targets
}

// holidayScheduleMinutes sums the schedule Soll of the week's holiday days —
// the amount a holiday week's target shrinks by.
func holidayScheduleMinutes(entries []*WorkScheduleRow, staffAnchor *timezone.Date, weekStart timezone.Date, holidaySet map[timezone.Date]bool) int {
	total := 0
	for offset := 0; offset < 7; offset++ {
		day := weekStart.AddDays(offset)
		if !holidaySet[day] {
			continue
		}
		dayTarget, _ := DailyTargetFromSchedule(entries, staffAnchor, day)
		total += dayTarget
	}
	return total
}

// holidayTemplateMinutes is holidayScheduleMinutes for the work-time-template
// fallback path.
func holidayTemplateMinutes(template *WorkTimeTemplate, anchor timezone.Date, weekStart timezone.Date, holidaySet map[timezone.Date]bool) int {
	total := 0
	for offset := 0; offset < 7; offset++ {
		day := weekStart.AddDays(offset)
		if !holidaySet[day] {
			continue
		}
		dayTarget, _ := DailyTargetFromTemplate(template, anchor, day)
		total += dayTarget
	}
	return total
}
