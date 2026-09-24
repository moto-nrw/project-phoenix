package timetableplanning

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// WithNonWorkingDays skips statutory holidays and closing days (#3594).
func WithNonWorkingDays(calendar timetable.NonWorkingDayCalendar) MaterializationOption {
	return func(s *materializationService) { s.nonWorkingDays = calendar }
}

// timeframeIndex loads the tenant's timeframes in one query, keyed by ID. Both
// the materializer and the lost-edit probe resolve schedule times from it.
func (s *materializationService) timeframeIndex(ctx context.Context, op string) (map[int64]*schedule.Timeframe, error) {
	timeframes, err := s.timeframeRepo.ListAll(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: op, Err: err}
	}
	byID := make(map[int64]*schedule.Timeframe, len(timeframes))
	for _, tf := range timeframes {
		byID[tf.ID] = tf
	}
	return byID, nil
}

// candidatePeriod applies the per-date recurrence rules to one schedule row
// and counts why a candidate is passed over; nil means nothing materializes.
// expectedSlotsOn replays the same rules, so both stay one definition.
func (s *materializationService) candidatePeriod(
	tmpl *activities.Group,
	sch *activities.Schedule,
	date timezone.Date,
	periods []*schedule.CalendarPeriod,
	days timetable.NonWorkingDays,
	result *MaterializationResult,
) *schedule.CalendarPeriod {
	if sch.Weekday != isoWeekday(date) {
		return nil
	}
	if scheduleEndedOn(sch, date) || (tmpl.SeriesLastDay != nil && tmpl.SeriesLastDay.Before(date)) {
		result.CandidatesSkippedEnded++
		return nil
	}
	if scheduleNotStartedOn(sch, date) {
		result.CandidatesSkippedNotStarted++
		return nil
	}
	period := selectPeriod(tmpl, sch, date, periods, s.getLogger())
	switch {
	case period == nil:
		result.CandidatesSkippedNoPeriod++
	case !shouldMaterializeWeekPattern(sch.WeekPattern, date, period):
		result.CandidatesSkippedABWeek++
	case days.Skip(date.String(), tmpl.IncludeClosingDays) == timetable.SkipHoliday:
		result.CandidatesSkippedHoliday++
	case days.Skip(date.String(), tmpl.IncludeClosingDays) == timetable.SkipClosingDay:
		result.CandidatesSkippedClosingDay++
	default:
		return period
	}
	return nil
}
