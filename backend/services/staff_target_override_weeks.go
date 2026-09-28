package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

// staffTargetOverrideWeeks re-prices the history weeks a Sonderarbeitszeit
// touches (#3259) with the Monatskarte's own daily Soll, so the weekly summary
// cannot run a third copy of the resolution rules. The month service is built
// after the session service, so it is resolved at call time.
type staffTargetOverrideWeeks struct {
	overrides workforce.StaffTargetOverrideQuery
	months    func() timetracking.WorkTimeMonthService
}

func (w staffTargetOverrideWeeks) OverriddenWeeklyTargets(ctx context.Context, staffID int64, weekStarts []timezone.Date) (map[timezone.Date]int, error) {
	if len(weekStarts) == 0 {
		return nil, nil
	}
	from, to := weekStarts[0], weekStarts[0]
	for _, weekStart := range weekStarts[1:] {
		from, to = min(from, weekStart), max(to, weekStart)
	}
	to = to.AddDays(6)
	days, err := w.overrides.StaffTargetOverrideDays(ctx, []int64{staffID}, from.String(), to.String())
	if err != nil || len(days[staffID]) == 0 {
		return nil, err
	}
	targets, err := w.months().GetDailyTargets(ctx, staffID, from, to)
	if err != nil {
		return nil, err
	}
	perDay := make(map[timezone.Date]int, len(targets))
	for _, target := range targets {
		perDay[target.Date] = target.TargetMinutes
	}
	result := make(map[timezone.Date]int)
	for _, weekStart := range weekStarts {
		touched, total := false, 0
		for offset := range 7 {
			day := weekStart.AddDays(offset)
			_, overridden := days[staffID][day.String()]
			touched = touched || overridden
			total += perDay[day]
		}
		if touched {
			result[weekStart] = total
		}
	}
	return result, nil
}
