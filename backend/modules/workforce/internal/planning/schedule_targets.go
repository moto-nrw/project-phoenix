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
