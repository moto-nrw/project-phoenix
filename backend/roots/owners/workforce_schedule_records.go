package owners

import (
	"context"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// WorkSessionScheduleRecords is the retained staff-work-schedule repository
// contract the schedule port is served from.
type WorkSessionScheduleRecords interface {
	GetCurrentByStaffID(context.Context, int64) ([]*config.StaffWorkSchedule, error)
	FindByStaffIDsValidInRange(context.Context, []int64, config.CalendarDate, config.CalendarDate) ([]*config.StaffWorkSchedule, error)
	ReplaceSchedule(context.Context, int64, []*config.StaffWorkSchedule, config.CalendarDate) error
	ReplaceScheduleWithValidFrom(context.Context, int64, []*config.StaffWorkSchedule, config.CalendarDate, config.CalendarDate) error
}

type WorkSessionSchedules struct{ records WorkSessionScheduleRecords }

func NewWorkSessionSchedules(records WorkSessionScheduleRecords) *WorkSessionSchedules {
	return &WorkSessionSchedules{records: records}
}

func (r *WorkSessionSchedules) GetCurrentByStaffID(ctx context.Context, staffID int64) ([]*workforceCompose.WorkScheduleRow, error) {
	rows, err := r.records.GetCurrentByStaffID(ctx, staffID)
	return workScheduleRows(rows), err
}

func (r *WorkSessionSchedules) FindByStaffIDsValidInRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) ([]*workforceCompose.WorkScheduleRow, error) {
	rows, err := r.records.FindByStaffIDsValidInRange(ctx, staffIDs, config.CalendarDate(from), config.CalendarDate(to))
	return workScheduleRows(rows), err
}

func (r *WorkSessionSchedules) ReplaceSchedule(ctx context.Context, staffID int64, rows []*workforceCompose.WorkScheduleRow, anchor timezone.Date) error {
	return r.records.ReplaceSchedule(ctx, staffID, staffWorkScheduleRows(rows), config.CalendarDate(anchor))
}

func (r *WorkSessionSchedules) ReplaceScheduleWithValidFrom(ctx context.Context, staffID int64, rows []*workforceCompose.WorkScheduleRow, anchor, validFrom timezone.Date) error {
	return r.records.ReplaceScheduleWithValidFrom(ctx, staffID, staffWorkScheduleRows(rows), config.CalendarDate(anchor), config.CalendarDate(validFrom))
}

// WorkSessionTimeModelRecords is the retained work-time-model repository
// contract the template port is served from.
type WorkSessionTimeModelRecords interface {
	FindByID(context.Context, int64) (*config.WorkTimeModel, error)
	Create(context.Context, *config.WorkTimeModel, []*config.WorkTimeModelEntry) error
}

type WorkSessionTimeModels struct{ records WorkSessionTimeModelRecords }

func NewWorkSessionTimeModels(records WorkSessionTimeModelRecords) *WorkSessionTimeModels {
	return &WorkSessionTimeModels{records: records}
}

func (r *WorkSessionTimeModels) FindByID(ctx context.Context, id int64) (*workforceCompose.WorkTimeTemplate, error) {
	row, err := r.records.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return workTimeTemplate(row), nil
}

// Create persists the template and its entries and writes the assigned
// identity back onto the caller's value, which the service binds to the staff
// member afterwards.
func (r *WorkSessionTimeModels) Create(ctx context.Context, template *workforceCompose.WorkTimeTemplate, entries []*workforceCompose.WorkTimeTemplateEntry) error {
	if template == nil {
		return nil
	}
	row := &config.WorkTimeModel{
		Name:               template.Name,
		RotationLength:     template.RotationLength,
		RotationAnchorDate: config.CalendarDate(template.RotationAnchorDate),
	}
	if err := r.records.Create(ctx, row, workTimeModelEntries(entries)); err != nil {
		return err
	}
	template.ID = row.ID
	return nil
}

// WorkScheduleTargetRecords is the retained staff-work-schedule read the
// target port is served from.
type WorkScheduleTargetRecords interface {
	FindByStaffIDsValidInRange(context.Context, []int64, config.CalendarDate, config.CalendarDate) ([]*config.StaffWorkSchedule, error)
	HasScheduleHistory(context.Context, int64) (bool, error)
	FindStaffIDsWithScheduleHistory(context.Context, []int64) (map[int64]bool, error)
}

// ScheduleTargetRows serves the daily targets of the schedule rows.
type ScheduleTargetRows struct{ records WorkScheduleTargetRecords }

func NewWorkScheduleTargets(records WorkScheduleTargetRecords) *ScheduleTargetRows {
	return &ScheduleTargetRows{records: records}
}

func (r *ScheduleTargetRows) TargetsForStaff(ctx context.Context, id int64, from, to timezone.Date) (workforceCompose.WorkScheduleTargets, error) {
	rows, err := r.records.FindByStaffIDsValidInRange(ctx, []int64{id}, config.CalendarDate(from), config.CalendarDate(to))
	return workScheduleTargets(rows), err
}

func (r *ScheduleTargetRows) TargetsByStaff(ctx context.Context, ids []int64, from, to timezone.Date) (map[int64]workforceCompose.WorkScheduleTargets, error) {
	rows, err := r.records.FindByStaffIDsValidInRange(ctx, ids, config.CalendarDate(from), config.CalendarDate(to))
	if err != nil {
		return nil, err
	}
	byStaff := make(map[int64][]*config.StaffWorkSchedule, len(ids))
	for _, row := range rows {
		byStaff[row.StaffID] = append(byStaff[row.StaffID], row)
	}
	result := make(map[int64]workforceCompose.WorkScheduleTargets, len(byStaff))
	for staffID, entries := range byStaff {
		result[staffID] = workScheduleTargets(entries)
	}
	return result, nil
}

func (r *ScheduleTargetRows) HasScheduleHistory(ctx context.Context, id int64) (bool, error) {
	return r.records.HasScheduleHistory(ctx, id)
}

func (r *ScheduleTargetRows) FindStaffIDsWithScheduleHistory(ctx context.Context, ids []int64) (map[int64]bool, error) {
	return r.records.FindStaffIDsWithScheduleHistory(ctx, ids)
}

func workScheduleTargets(rows []*config.StaffWorkSchedule) workforceCompose.WorkScheduleTargets {
	return workforceCompose.WorkScheduleTargets{HasEntries: len(rows) > 0, DailyTarget: func(anchor *timezone.Date, date timezone.Date) (int, bool) {
		var legacyAnchor *config.CalendarDate
		if anchor != nil {
			value := config.CalendarDate(*anchor)
			legacyAnchor = &value
		}
		return config.DailyTargetFromSchedule(rows, legacyAnchor, config.CalendarDate(date))
	}}
}

// WorkTimeTargetModelRecords is the retained work-time-model read the target
// model port is served from.
type WorkTimeTargetModelRecords interface {
	FindByID(context.Context, int64) (*config.WorkTimeModel, error)
	FindByIDs(context.Context, []int64) ([]*config.WorkTimeModel, error)
}

type WorkTimeTargetModels struct{ records WorkTimeTargetModelRecords }

func NewWorkTimeTargetModels(records WorkTimeTargetModelRecords) *WorkTimeTargetModels {
	return &WorkTimeTargetModels{records: records}
}

func (r *WorkTimeTargetModels) FindByID(ctx context.Context, id int64) (*workforceCompose.WorkTimeTargetModel, error) {
	row, err := r.records.FindByID(ctx, id)
	return workTimeTargetModel(row), err
}

func (r *WorkTimeTargetModels) FindByIDs(ctx context.Context, ids []int64) ([]*workforceCompose.WorkTimeTargetModel, error) {
	rows, err := r.records.FindByIDs(ctx, ids)
	if rows == nil {
		return nil, err
	}
	result := make([]*workforceCompose.WorkTimeTargetModel, len(rows))
	for i, row := range rows {
		result[i] = workTimeTargetModel(row)
	}
	return result, err
}

func workTimeTargetModel(row *config.WorkTimeModel) *workforceCompose.WorkTimeTargetModel {
	if row == nil {
		return nil
	}
	return &workforceCompose.WorkTimeTargetModel{
		ID: row.ID, RotationAnchorDate: timezone.Date(row.RotationAnchorDate),
		DailyTarget: func(anchor, date timezone.Date) (int, bool) {
			return config.DailyTargetFromModel(row, config.CalendarDate(anchor), config.CalendarDate(date))
		},
	}
}

func workScheduleRows(rows []*config.StaffWorkSchedule) []*workforceCompose.WorkScheduleRow {
	if rows == nil {
		return nil
	}
	converted := make([]*workforceCompose.WorkScheduleRow, len(rows))
	for index, row := range rows {
		converted[index] = workScheduleRow(row)
	}
	return converted
}

func workScheduleRow(row *config.StaffWorkSchedule) *workforceCompose.WorkScheduleRow {
	if row == nil {
		return nil
	}
	return &workforceCompose.WorkScheduleRow{
		StaffID:            row.StaffID,
		WeekIndex:          row.WeekIndex,
		RotationLength:     row.RotationLength,
		DayOfWeek:          row.DayOfWeek,
		TargetMinutes:      row.TargetMinutes,
		StartTime:          row.StartTime,
		RotationAnchorDate: scheduleDatePointer(row.RotationAnchorDate),
		ValidFrom:          timezone.Date(row.ValidFrom),
		ValidUntil:         scheduleDatePointer(row.ValidUntil),
	}
}

func staffWorkScheduleRows(rows []*workforceCompose.WorkScheduleRow) []*config.StaffWorkSchedule {
	if rows == nil {
		return nil
	}
	converted := make([]*config.StaffWorkSchedule, len(rows))
	for index, row := range rows {
		if row == nil {
			continue
		}
		converted[index] = &config.StaffWorkSchedule{
			StaffID:            row.StaffID,
			WeekIndex:          row.WeekIndex,
			RotationLength:     row.RotationLength,
			DayOfWeek:          row.DayOfWeek,
			TargetMinutes:      row.TargetMinutes,
			StartTime:          row.StartTime,
			RotationAnchorDate: calendarDatePointer(row.RotationAnchorDate),
			ValidFrom:          config.CalendarDate(row.ValidFrom),
			ValidUntil:         calendarDatePointer(row.ValidUntil),
		}
	}
	return converted
}

func workTimeTemplate(row *config.WorkTimeModel) *workforceCompose.WorkTimeTemplate {
	if row == nil {
		return nil
	}
	return &workforceCompose.WorkTimeTemplate{
		ID:                 row.ID,
		Name:               row.Name,
		RotationLength:     row.RotationLength,
		RotationAnchorDate: timezone.Date(row.RotationAnchorDate),
		Entries:            workTimeTemplateEntries(row.Entries),
	}
}

func workTimeTemplateEntries(entries []*config.WorkTimeModelEntry) []*workforceCompose.WorkTimeTemplateEntry {
	if entries == nil {
		return nil
	}
	converted := make([]*workforceCompose.WorkTimeTemplateEntry, len(entries))
	for index, entry := range entries {
		if entry == nil {
			continue
		}
		converted[index] = &workforceCompose.WorkTimeTemplateEntry{
			WeekIndex:     entry.WeekIndex,
			DayOfWeek:     entry.DayOfWeek,
			TargetMinutes: entry.TargetMinutes,
			StartTime:     entry.StartTime,
		}
	}
	return converted
}

func workTimeModelEntries(entries []*workforceCompose.WorkTimeTemplateEntry) []*config.WorkTimeModelEntry {
	if entries == nil {
		return nil
	}
	converted := make([]*config.WorkTimeModelEntry, len(entries))
	for index, entry := range entries {
		if entry == nil {
			continue
		}
		converted[index] = &config.WorkTimeModelEntry{
			WeekIndex:     entry.WeekIndex,
			DayOfWeek:     entry.DayOfWeek,
			TargetMinutes: entry.TargetMinutes,
			StartTime:     entry.StartTime,
		}
	}
	return converted
}

func scheduleDatePointer(date *config.CalendarDate) *timezone.Date {
	if date == nil {
		return nil
	}
	converted := timezone.Date(*date)
	return &converted
}

func calendarDatePointer(date *timezone.Date) *config.CalendarDate {
	if date == nil {
		return nil
	}
	converted := config.CalendarDate(*date)
	return &converted
}
