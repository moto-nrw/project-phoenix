package ports

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The retained row adapters the roots name. The Timetable application serves
// them over the Care Plan and Workforce records and the calendar period
// references (#4084); the composition builds them.

// CareArrivalScheduleRows serve the retained arrival schedules from Care Plan.
type CareArrivalScheduleRows interface {
	StudentArrivalScheduleRecords
}

// CareArrivalExceptionRows serve the retained arrival exceptions from Care
// Plan.
type CareArrivalExceptionRows interface {
	StudentArrivalExceptionRecords
}

// CarePickupScheduleRows serve the retained pickup schedules from Care Plan.
type CarePickupScheduleRows interface {
	StudentPickupScheduleRecords
}

// CarePickupExceptionRows serve the retained pickup exceptions from Care
// Plan.
type CarePickupExceptionRows interface {
	StudentPickupExceptionRecords
}

// CarePickupNoteRows serve the retained pickup notes from Care Plan.
type CarePickupNoteRows interface {
	StudentPickupNoteRecords
}

// CareScheduleRequestRows serve the retained care schedule change requests
// from Care Plan.
type CareScheduleRequestRows interface {
	CareScheduleChangeRequestRecords
}

// CalendarPeriodUsageRepository counts, per calendar period, the rows that
// reference it.
type CalendarPeriodUsageRepository interface {
	Usage(ctx context.Context) (map[int64]CalendarPeriodUsage, error)
	UsageCounts(ctx context.Context) (map[int64]scheduleModels.CalendarPeriodUsage, error)
}

// CallerActivityGroups read the activity groups a caller supervises.
type CallerActivityGroups interface {
	ActivityGroupRows(ctx context.Context, ids []int64) (any, error)
	SupervisedActivityGroupIDs(ctx context.Context, staffID int64) ([]int64, error)
}

// WorkforceShiftRows serve the retained staff shifts from Workforce.
type WorkforceShiftRows interface {
	Create(ctx context.Context, shift *scheduleModels.StaffShift) error
	Delete(ctx context.Context, id int64) error
	FindByDateRange(ctx context.Context, start, end calendar.Date) ([]*scheduleModels.StaffShift, error)
	FindByID(ctx context.Context, id int64) (*scheduleModels.StaffShift, error)
	FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end calendar.Date) ([]*scheduleModels.StaffShift, error)
	FindByStaffIDsAndDates(ctx context.Context, staffIDs []int64, dates []calendar.Date) ([]*scheduleModels.StaffShift, error)
	FindUsedCalendarWeeks(ctx context.Context, start, end calendar.Date) ([]calendar.Date, error)
	Update(ctx context.Context, shift *scheduleModels.StaffShift) error
}

// WorkforceShiftTypeRows serve the retained shift types from Workforce.
type WorkforceShiftTypeRows interface {
	Create(ctx context.Context, shiftType *scheduleModels.ShiftType) error
	Delete(ctx context.Context, id int64) error
	ListAll(ctx context.Context) ([]*scheduleModels.ShiftType, error)
}
