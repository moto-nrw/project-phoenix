package timetracking

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// TargetOverrideReader is the consumer port over the Workforce
// Sonderarbeitszeiten (#3259): the days they set a target on, already without
// weekends and statutory holidays.
type TargetOverrideReader interface {
	StaffTargetOverrideDays(ctx context.Context, staffIDs []int64, from, to string) (workforce.TargetOverrideDays, error)
}

// dailyTargetResolver is the Workforce day-Soll order (override, non-working
// day, schedule, template); this service only loads its inputs.
type dailyTargetResolver = workforce.DayTargetResolver

func (s *workTimeMonthService) buildTargetResolver(ctx context.Context, staffID int64, from, to timezone.Date) (*dailyTargetResolver, error) {
	resolver := &dailyTargetResolver{}

	if s.holidayReader != nil {
		holidaySet, err := s.holidayReader.HolidayDates(ctx, from, to)
		if err != nil {
			return nil, fmt.Errorf("failed to load public holidays: %w", err)
		}
		resolver.NonWorkingDays = holidaySet
	}
	if s.overrideReader != nil {
		days, err := s.overrideReader.StaffTargetOverrideDays(ctx, []int64{staffID}, from.String(), to.String())
		if err != nil {
			return nil, fmt.Errorf("failed to load target overrides: %w", err)
		}
		resolver.Overrides = days[staffID]
	}

	staff, err := s.staffRepo.ScheduleAssignment(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to load staff for month summary: %w", err)
	}
	entries, err := s.scheduleRepo.TargetsForStaff(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to load work schedules: %w", err)
	}
	if entries.HasEntries {
		resolver.Schedule = func(d timezone.Date) (int, bool) { return entries.DailyTarget(staff.RotationAnchorDate, d) }
	}
	if entries.HasEntries || staff.WorkTimeModelID == nil {
		return resolver, nil
	}

	// No snapshot is valid in this range. The assigned work-time model may only
	// stand in when the staff member has NO snapshot at all: if versions exist
	// but none covers these days, the range lies before the first one (or after
	// the schedule was emptied) and the Soll is genuinely zero. Applying the
	// current model there would charge Soll for days the schedule didn't exist
	// yet — and would make the answer depend on the query window, since a range
	// that also touches the first snapshot resolves the very same days to zero.
	hasHistory, err := s.scheduleRepo.HasScheduleHistory(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to check schedule history: %w", err)
	}
	if hasHistory {
		return resolver, nil
	}

	model, err := s.workModelRepo.FindByID(ctx, *staff.WorkTimeModelID)
	if err != nil {
		return nil, fmt.Errorf("failed to load work time model: %w", err)
	}
	anchor := model.RotationAnchorDate
	if staff.RotationAnchorDate != nil {
		anchor = *staff.RotationAnchorDate
	}
	resolver.Template = func(d timezone.Date) (int, bool) { return model.DailyTarget(anchor, d) }
	return resolver, nil
}
