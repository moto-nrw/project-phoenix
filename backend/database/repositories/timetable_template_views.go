package repositories

import (
	"context"
	"errors"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func (r timetableActivityGroupRepository) ListTemplateRows(ctx context.Context, templateID *int64) ([]activitiesModels.TemplateListRow, error) {
	rows, err := r.timetable.ListTemplateRows(ctx, templateID)
	return r.legacyTemplateRows(ctx, rows, err)
}

func (r timetableActivityGroupRepository) ListTemplateRowsForTemplatePeriod(ctx context.Context, templateID, periodID int64) ([]activitiesModels.TemplateListRow, error) {
	rows, err := r.timetable.ListTemplateRowsForTemplatePeriod(ctx, templateID, periodID)
	return r.legacyTemplateRows(ctx, rows, err)
}

func (r timetableActivityGroupRepository) ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]activitiesModels.TemplateListRow, error) {
	rows, err := r.timetable.ListTemplateRowsForPeriod(ctx, periodID)
	return r.legacyTemplateRows(ctx, rows, err)
}

func (r timetableActivityGroupRepository) ListTemplateWeekdayRoster(ctx context.Context, templateID, periodID *int64) ([]activitiesModels.TemplateWeekdayRosterRow, error) {
	rows, err := r.timetable.ListTemplateWeekdayRoster(ctx, templateID, periodID)
	if err != nil {
		return nil, err
	}
	result := make([]activitiesModels.TemplateWeekdayRosterRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, activitiesModels.TemplateWeekdayRosterRow{
			TemplateID: row.TemplateID, Weekday: row.Weekday, Kind: row.Kind,
			PersonID: row.PersonID, IsPrimary: row.IsPrimary,
		})
	}
	return result, nil
}

func (r timetableActivityGroupRepository) ListTemplateCapacityOccurrences(ctx context.Context, periodID *int64, templateIDs []int64) ([]activitiesModels.TemplateCapacityOccurrence, error) {
	if r.calendar == nil {
		return nil, errors.New("timetable compatibility: school calendar is required")
	}
	periods, err := r.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{ActiveOnly: true})
	if err != nil {
		return nil, err
	}
	ownerPeriods := make([]timetable.TemplateCapacityPeriod, 0, len(periods))
	for _, period := range periods {
		ownerPeriods = append(ownerPeriods, timetable.TemplateCapacityPeriod{
			ID: period.ID, TenantID: period.TenantID, StartDate: period.StartDate,
			EndDate: period.EndDate, WeekCycleLength: period.WeekCycleLength,
			WeekCycleAnchor: period.WeekCycleAnchor,
		})
	}
	rows, err := r.timetable.ListTemplateCapacityOccurrences(ctx, periodID, templateIDs, ownerPeriods)
	if err != nil {
		return nil, err
	}
	result := make([]activitiesModels.TemplateCapacityOccurrence, 0, len(rows))
	for _, row := range rows {
		value := activitiesModels.TemplateCapacityOccurrence{
			TemplateID: row.TemplateID, CalendarPeriodID: row.CalendarPeriodID,
			EnrollmentCount: row.EnrollmentCount, SupervisorCount: row.SupervisorCount,
		}
		setDate(&value.OccurrenceDate, row.OccurrenceDate)
		result = append(result, value)
	}
	return result, nil
}

func (r timetableActivityGroupRepository) legacyTemplateRows(ctx context.Context, rows []timetable.TemplateListRow, queryErr error) ([]activitiesModels.TemplateListRow, error) {
	if queryErr != nil {
		return nil, queryErr
	}
	if err := r.attachTemplateOwnerNames(ctx, rows); err != nil {
		return nil, err
	}
	result := make([]activitiesModels.TemplateListRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, legacyTemplateRow(row))
	}
	return result, nil
}

func (r timetableActivityGroupRepository) attachTemplateOwnerNames(ctx context.Context, rows []timetable.TemplateListRow) error {
	if r.rooms == nil || r.shiftTypes == nil {
		return errors.New("timetable compatibility: room and shift-type directories are required")
	}
	roomIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.RoomID != nil {
			roomIDs = append(roomIDs, *row.RoomID)
		}
	}
	rooms, err := r.rooms.ListRoomsByID(ctx, dedupeTemplateIDs(roomIDs))
	if err != nil {
		return err
	}
	shifts, err := r.shiftTypes.ListAll(ctx)
	if err != nil {
		return err
	}
	roomNames := make(map[int64]string, len(rooms))
	for _, room := range rooms {
		roomNames[room.ID] = room.Name
	}
	shiftByID := make(map[int64]struct{ name, color string }, len(shifts))
	for _, shift := range shifts {
		shiftByID[shift.ID] = struct{ name, color string }{shift.Name, shift.Color}
	}
	for index := range rows {
		if rows[index].RoomID != nil {
			name := roomNames[*rows[index].RoomID]
			rows[index].RoomName = &name
		}
		if rows[index].ShiftTypeID != nil {
			shift := shiftByID[*rows[index].ShiftTypeID]
			rows[index].ShiftTypeName, rows[index].ShiftTypeColor = shift.name, shift.color
		}
	}
	return nil
}

func legacyTemplateRow(row timetable.TemplateListRow) activitiesModels.TemplateListRow {
	return activitiesModels.TemplateListRow{
		TemplateID: row.TemplateID, Name: row.Name, Type: row.Type,
		CategoryID: row.CategoryID, CategoryName: row.CategoryName,
		PlanningTrackID: legacyNullInt64(row.PlanningTrackID), PlanningTrackName: row.PlanningTrackName,
		PlanningTrackColor: row.PlanningTrackColor, PlanningTrackOrder: legacyNullInt64(row.PlanningTrackOrder),
		RoomID: legacyNullInt64(row.RoomID), RoomName: legacyNullString(row.RoomName),
		EducationGroupID: legacyNullInt64(row.EducationGroupID), EducationGroupName: legacyNullString(row.EducationGroupName),
		IsOpen: row.IsOpen, MaxParticipants: row.MaxParticipants, RequiredStaff: legacyNullInt64(row.RequiredStaff),
		TemplateCalendarPeriodID: legacyNullInt64(row.TemplateCalendarPeriodID), TargetGroupType: row.TargetGroupType,
		TargetGradeLevel: legacyNullInt16(row.TargetGradeLevel), TargetSchoolClass: legacyNullString(row.TargetSchoolClass),
		SourceCareOfferingIDsJSON: row.SourceCareOfferingIDsJSON, SourceGradeLevelsJSON: row.SourceGradeLevelsJSON,
		SourceSchoolClassesJSON: row.SourceSchoolClassesJSON, ListKind: legacyNullString(row.ListKind), Notes: legacyNullString(row.Notes),
		ShiftTypeID: legacyNullInt64(row.ShiftTypeID), ShiftTypeName: row.ShiftTypeName, ShiftTypeColor: row.ShiftTypeColor,
		EnrollmentCount: row.EnrollmentCount, SupervisorCount: row.SupervisorCount,
		CapacityEnrollmentCount: row.CapacityEnrollmentCount, CapacitySupervisorCount: row.CapacitySupervisorCount,
		CapacityOccurrenceFound: row.CapacityOccurrenceFound, StudentIDs: row.StudentIDs, StaffIDs: row.StaffIDs,
		PrimaryStaffID: legacyNullInt64(row.PrimaryStaffID), ScheduleID: row.ScheduleID, Weekday: row.Weekday,
		StartTime: legacyNullString(row.StartTime), EndTime: legacyNullString(row.EndTime), WeekPattern: row.WeekPattern,
		CalendarPeriodID: legacyNullInt64(row.CalendarPeriodID), ScheduleValidFrom: legacyNullString(row.ScheduleValidFrom),
		ScheduleValidUntil: legacyNullString(row.ScheduleValidUntil),
	}
}

func dedupeTemplateIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
