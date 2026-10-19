package application

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The staff calendar feed reads the retained Timetable and Dienstplan rows
// through the root, which maps the records below into the School Calendar
// portal's vocabulary. A nil row listing stays nil.

// FindCalendarAssignments lists the staff member's assignments in the range.
func FindCalendarAssignments(ctx context.Context, rows ports.CalendarAssignmentRows, staffID int64, from, to string) ([]ports.CalendarAssignment, error) {
	values, err := rows.FindByStaffAndDateRange(ctx, staffID, calendar.Date(from), calendar.Date(to))
	return calendarRecords(values, func(v *scheduleModels.InstanceStaff) ports.CalendarAssignment {
		return ports.CalendarAssignment{InstanceID: v.InstanceID, RoomID: v.RoomID, UpdatedAt: v.UpdatedAt}
	}), err
}

// FindCalendarInstances lists the instances by ID.
func FindCalendarInstances(ctx context.Context, rows ports.CalendarInstanceRows, ids []int64) ([]ports.CalendarInstance, error) {
	values, err := rows.FindByIDs(ctx, ids)
	return calendarRecords(values, func(v *scheduleModels.ActivityInstance) ports.CalendarInstance {
		return ports.CalendarInstance{
			ID: v.ID, RoomID: v.RoomID, Title: v.Title, Status: v.Status, Description: v.Description, Date: string(v.Date),
			StartTime: v.StartTime, EndTime: v.EndTime, UpdatedAt: v.UpdatedAt, TemplateType: v.TemplateType,
		}
	}), err
}

// FindCalendarShifts lists the staff member's planned shifts in the range.
func FindCalendarShifts(ctx context.Context, rows ports.CalendarShiftRows, staffID int64, from, to string) ([]ports.CalendarShift, error) {
	values, err := rows.FindByStaffAndDateRange(ctx, staffID, calendar.Date(from), calendar.Date(to))
	return calendarRecords(values, func(v *scheduleModels.StaffShift) ports.CalendarShift {
		return ports.CalendarShift{
			ID: v.ID, Date: string(v.Date), StartTime: v.StartTime, EndTime: v.EndTime, UpdatedAt: v.UpdatedAt,
			ShiftTypeID: v.ShiftTypeID, Cancelled: v.Cancelled, Notes: v.Notes,
		}
	}), err
}

// ListCalendarShiftTypes names every Schichtart.
func ListCalendarShiftTypes(ctx context.Context, rows ports.CalendarShiftTypeRows) ([]ports.CalendarShiftType, error) {
	values, err := rows.ListAll(ctx)
	return calendarRecords(values, func(v *scheduleModels.ShiftType) ports.CalendarShiftType {
		return ports.CalendarShiftType{ID: v.ID, Name: v.Name}
	}), err
}

func calendarRecords[A, B any](values []A, convert func(A) B) []B {
	if values == nil {
		return nil
	}
	result := make([]B, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}
