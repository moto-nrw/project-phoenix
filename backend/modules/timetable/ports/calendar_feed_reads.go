package ports

import (
	"context"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CalendarShiftRows are a staff member's planned shifts.
type CalendarShiftRows interface {
	FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end calendar.Date) ([]*scheduleModels.StaffShift, error)
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
