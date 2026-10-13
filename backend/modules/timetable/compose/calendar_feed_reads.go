package compose

import (
	"context"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// The staff calendar feed reads the retained Timetable and Dienstplan rows
// through the root, which maps the records below into the School Calendar
// portal's vocabulary. A nil row listing stays nil.

// CalendarAssignmentRows are a staff member's instance assignments.
type CalendarAssignmentRows interface {
	FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to scheduleModels.Date) ([]*scheduleModels.InstanceStaff, error)
}

// CalendarInstanceRows are the activity instances by ID.
type CalendarInstanceRows interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*scheduleModels.ActivityInstance, error)
}

// CalendarShiftRows are a staff member's planned shifts.
type CalendarShiftRows interface {
	FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error)
}

// CalendarShiftTypeRows are the Schichtarten the feed labels shifts with.
type CalendarShiftTypeRows interface {
	ListAll(ctx context.Context) ([]*scheduleModels.ShiftType, error)
}

// CalendarAssignment is one instance a staff member is assigned to.
type CalendarAssignment struct {
	InstanceID int64
	RoomID     *int64
	UpdatedAt  time.Time
}

// CalendarInstance is one activity instance of the feed.
type CalendarInstance struct {
	ID           int64
	RoomID       int64
	Title        string
	Status       string
	Description  *string
	Date         string
	StartTime    time.Time
	EndTime      time.Time
	UpdatedAt    time.Time
	TemplateType string
}

// CalendarShift is one planned shift of the feed.
type CalendarShift struct {
	ID          int64
	Date        string
	StartTime   time.Time
	EndTime     time.Time
	UpdatedAt   time.Time
	ShiftTypeID *int64
	Cancelled   bool
	Notes       string
}

// CalendarShiftType names one Schichtart.
type CalendarShiftType struct {
	ID   int64
	Name string
}

// FindCalendarAssignments lists the staff member's assignments in the range.
func FindCalendarAssignments(ctx context.Context, rows CalendarAssignmentRows, staffID int64, from, to string) ([]CalendarAssignment, error) {
	values, err := rows.FindByStaffAndDateRange(ctx, staffID, scheduleModels.Date(from), scheduleModels.Date(to))
	return calendarRecords(values, func(v *scheduleModels.InstanceStaff) CalendarAssignment {
		return CalendarAssignment{InstanceID: v.InstanceID, RoomID: v.RoomID, UpdatedAt: v.UpdatedAt}
	}), err
}

// FindCalendarInstances lists the instances by ID.
func FindCalendarInstances(ctx context.Context, rows CalendarInstanceRows, ids []int64) ([]CalendarInstance, error) {
	values, err := rows.FindByIDs(ctx, ids)
	return calendarRecords(values, func(v *scheduleModels.ActivityInstance) CalendarInstance {
		return CalendarInstance{
			ID: v.ID, RoomID: v.RoomID, Title: v.Title, Status: v.Status, Description: v.Description, Date: string(v.Date),
			StartTime: v.StartTime, EndTime: v.EndTime, UpdatedAt: v.UpdatedAt, TemplateType: v.TemplateType,
		}
	}), err
}

// FindCalendarShifts lists the staff member's planned shifts in the range.
func FindCalendarShifts(ctx context.Context, rows CalendarShiftRows, staffID int64, from, to string) ([]CalendarShift, error) {
	values, err := rows.FindByStaffAndDateRange(ctx, staffID, scheduleModels.Date(from), scheduleModels.Date(to))
	return calendarRecords(values, func(v *scheduleModels.StaffShift) CalendarShift {
		return CalendarShift{
			ID: v.ID, Date: string(v.Date), StartTime: v.StartTime, EndTime: v.EndTime, UpdatedAt: v.UpdatedAt,
			ShiftTypeID: v.ShiftTypeID, Cancelled: v.Cancelled, Notes: v.Notes,
		}
	}), err
}

// ListCalendarShiftTypes names every Schichtart.
func ListCalendarShiftTypes(ctx context.Context, rows CalendarShiftTypeRows) ([]CalendarShiftType, error) {
	values, err := rows.ListAll(ctx)
	return calendarRecords(values, func(v *scheduleModels.ShiftType) CalendarShiftType {
		return CalendarShiftType{ID: v.ID, Name: v.Name}
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
