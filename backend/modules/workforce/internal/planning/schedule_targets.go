package planning

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The weekly targets are priced with Workforce's own Soll arithmetic over its
// schedule rows and templates; these helpers translate the capability's
// schedule versions and work-time templates into that vocabulary.

// scheduleRows translates schedule versions into Soll rows.
func scheduleRows(schedules []workforce.StaffWorkSchedule) []*workforce.ScheduleRow {
	rows := make([]*workforce.ScheduleRow, 0, len(schedules))
	for _, schedule := range schedules {
		row := &workforce.ScheduleRow{
			StaffID: schedule.StaffID, WeekIndex: schedule.WeekIndex, RotationLength: schedule.RotationLength,
			DayOfWeek: schedule.DayOfWeek, TargetMinutes: schedule.TargetMinutes, ValidFrom: calendar.Date(schedule.ValidFrom),
		}
		if schedule.RotationAnchorDate != "" {
			anchor := calendar.Date(schedule.RotationAnchorDate)
			row.RotationAnchorDate = &anchor
		}
		if schedule.ValidUntil != "" {
			until := calendar.Date(schedule.ValidUntil)
			row.ValidUntil = &until
		}
		rows = append(rows, row)
	}
	return rows
}

// scheduleRowsByStaff groups Soll rows by staff member.
func scheduleRowsByStaff(rows []*workforce.ScheduleRow) map[int64][]*workforce.ScheduleRow {
	byStaff := make(map[int64][]*workforce.ScheduleRow)
	for _, row := range rows {
		if row != nil {
			byStaff[row.StaffID] = append(byStaff[row.StaffID], row)
		}
	}
	return byStaff
}

// loadWorkTimeTemplates resolves the given work-time templates, keyed by ID.
func (s *staffScheduleOverviewService) loadWorkTimeTemplates(ctx context.Context, ids []int64) (map[int64]*workforce.ScheduleTemplate, error) {
	models, err := s.deps.WorkModels.ListWorkTimeModelsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load work-time models: %w", err)
	}
	templates := make(map[int64]*workforce.ScheduleTemplate, len(models))
	for _, model := range models {
		template := &workforce.ScheduleTemplate{
			ID: model.ID, Name: model.Name, RotationLength: model.RotationLength,
			RotationAnchorDate: calendar.Date(model.RotationAnchorDate),
			Entries:            make([]*workforce.ScheduleTemplateEntry, 0, len(model.Entries)),
		}
		for _, entry := range model.Entries {
			template.Entries = append(template.Entries, &workforce.ScheduleTemplateEntry{
				WeekIndex: entry.WeekIndex, DayOfWeek: entry.DayOfWeek, TargetMinutes: entry.TargetMinutes,
			})
		}
		templates[model.ID] = template
	}
	return templates, nil
}

// templateAnchor is the rotation anchor a template is priced from: the staff
// member's own anchor when set, the template's otherwise.
func templateAnchor(member *workforce.PlanStaff, template *workforce.ScheduleTemplate) calendar.Date {
	if member.RotationAnchorDate != nil {
		return *member.RotationAnchorDate
	}
	return template.RotationAnchorDate
}

// resolveWeeklyTargets mirrors the time-tracking Soll resolution: date-valid
// staff_work_schedules rows win per staff; the assigned work-time model is
// the fallback only when the schedule yields no target for any week at all.
func (s *staffScheduleOverviewService) resolveWeeklyTargets(
	ctx context.Context,
	staff []*workforce.PlanStaff,
	schedules []*workforce.ScheduleRow,
	weekStarts []calendar.Date,
) (map[staffDateKey]int, error) {
	targets := make(map[staffDateKey]int)

	holidaySet, err := s.holidayDatesForWeeks(ctx, weekStarts)
	if err != nil {
		return nil, err
	}

	entriesByStaff := scheduleRowsByStaff(schedules)

	residual := make([]*workforce.PlanStaff, 0)
	modelIDs := make([]int64, 0)
	seenModels := make(map[int64]struct{})
	for _, member := range staff {
		if member == nil {
			continue
		}
		found := addScheduleWeeklyTargets(targets, member, entriesByStaff[member.ID], weekStarts, holidaySet)
		if found || member.WorkTimeModelID == nil {
			continue
		}
		residual = append(residual, member)
		if _, exists := seenModels[*member.WorkTimeModelID]; !exists {
			seenModels[*member.WorkTimeModelID] = struct{}{}
			modelIDs = append(modelIDs, *member.WorkTimeModelID)
		}
	}

	if len(residual) == 0 || s.deps.WorkModels == nil {
		return targets, nil
	}
	modelsByID, err := s.loadWorkTimeTemplates(ctx, modelIDs)
	if err != nil {
		return nil, err
	}
	for _, member := range residual {
		if model := modelsByID[*member.WorkTimeModelID]; model != nil {
			addTemplateWeeklyTargets(targets, member, model, weekStarts, holidaySet)
		}
	}
	return targets, nil
}

// addScheduleWeeklyTargets stores the holiday-reduced schedule target of every
// week the member's schedule rows price, and reports whether any week had one.
func addScheduleWeeklyTargets(
	targets map[staffDateKey]int,
	member *workforce.PlanStaff,
	entries []*workforce.ScheduleRow,
	weekStarts []calendar.Date,
	holidaySet map[calendar.Date]bool,
) bool {
	if len(entries) == 0 {
		return false
	}
	dailyTarget := func(day calendar.Date) int {
		dayTarget, _ := workforce.DailyTargetFromSchedule(entries, member.RotationAnchorDate, day)
		return dayTarget
	}
	found := false
	for _, weekStart := range weekStarts {
		if target, ok := workforce.WeeklyTargetFromSchedule(entries, member.RotationAnchorDate, weekStart); ok {
			targets[staffDateKey{StaffID: member.ID, Date: weekStart}] = withoutHolidayTargets(target, weekStart, holidaySet, dailyTarget)
			found = true
		}
	}
	return found
}

// addTemplateWeeklyTargets stores the holiday-reduced work-time model target of
// every week the template prices for the member.
func addTemplateWeeklyTargets(
	targets map[staffDateKey]int,
	member *workforce.PlanStaff,
	model *workforce.ScheduleTemplate,
	weekStarts []calendar.Date,
	holidaySet map[calendar.Date]bool,
) {
	anchor := templateAnchor(member, model)
	dailyTarget := func(day calendar.Date) int {
		dayTarget, _ := workforce.DailyTargetFromTemplate(model, anchor, day)
		return dayTarget
	}
	for weekStart, target := range workforce.WeeklyTargetsFromTemplate(model, anchor, weekStarts) {
		targets[staffDateKey{StaffID: member.ID, Date: weekStart}] = withoutHolidayTargets(target, weekStart, holidaySet, dailyTarget)
	}
}

// withoutHolidayTargets subtracts the daily target of every public holiday in
// the week starting at weekStart from the weekly target.
func withoutHolidayTargets(target int, weekStart calendar.Date, holidaySet map[calendar.Date]bool, dailyTarget func(calendar.Date) int) int {
	for offset := 0; offset < 7; offset++ {
		day := weekStart.AddDays(offset)
		if holidaySet[day] {
			target -= dailyTarget(day)
		}
	}
	return target
}
