package planning

import (
	"context"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func weeksWithShifts(shifts []*StaffShift) []calendar.Date {
	seen := make(map[calendar.Date]bool)
	weeks := make([]calendar.Date, 0)
	for _, shift := range shifts {
		if shift == nil {
			continue
		}
		monday, _ := containingCalendarWeek(shift.Date)
		if !seen[monday] {
			seen[monday] = true
			weeks = append(weeks, monday)
		}
	}
	sort.Slice(weeks, func(i, j int) bool { return weeks[i].Before(weeks[j]) })
	return weeks
}

func (s *staffScheduleOverviewService) loadPlanningProfiles(ctx context.Context, staff []*workforce.PlanStaff, weeks []calendar.Date, from, to calendar.Date) ([]*workforce.ScheduleRow, error) {
	ids := overviewStaffIDs(staff)
	// Template bindings only price weekly summaries; an unused week has none.
	if len(ids) == 0 || len(weeks) == 0 {
		return nil, nil
	}
	if s.deps.PlanningProfiles == nil {
		if s.deps.WorkSchedules == nil {
			return nil, nil
		}
		rows, err := s.deps.WorkSchedules.StaffSchedulesInRange(ctx, ids, from.String(), to.String())
		return scheduleRows(rows), err
	}
	profiles, err := s.deps.PlanningProfiles.StaffPlanningProfiles(ctx, ids, from.String(), to.String())
	if err != nil {
		return nil, fmt.Errorf("load staff planning profiles: %w", err)
	}
	byID := make(map[int64]workforce.StaffPlanningProfile, len(profiles))
	var schedules []workforce.StaffWorkSchedule
	for _, profile := range profiles {
		byID[profile.MembershipID] = profile
		schedules = append(schedules, profile.Schedules...)
	}
	if err := applyPlanningBindings(staff, byID); err != nil {
		return nil, err
	}

	if s.deps.WorkSchedules == nil {
		return nil, nil
	}
	return scheduleRows(schedules), nil
}

func applyPlanningBindings(staff []*workforce.PlanStaff, byID map[int64]workforce.StaffPlanningProfile) error {
	for _, member := range staff {
		if member == nil {
			continue
		}
		profile := byID[member.ID]
		member.WorkTimeModelID = profile.WorkTimeModelID
		member.RotationAnchorDate = nil
		if profile.RotationAnchorDate != "" {
			date, err := calendar.ParseDate(profile.RotationAnchorDate)
			if err != nil {
				return err
			}
			member.RotationAnchorDate = &date
		}
	}
	return nil
}
