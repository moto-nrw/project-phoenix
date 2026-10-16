package ports

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// The retained row adapters the roots name. The Timetable application serves
// them over the Care Plan and Workforce records and the calendar period
// references (#4084); the composition builds them.

// CareArrivalScheduleRows serve the retained arrival schedules from Care Plan.
type CareArrivalScheduleRows interface {
	scheduleModels.StudentArrivalScheduleRepository
}

// CareArrivalExceptionRows serve the retained arrival exceptions from Care
// Plan.
type CareArrivalExceptionRows interface {
	scheduleModels.StudentArrivalExceptionRepository
}

// CarePickupScheduleRows serve the retained pickup schedules from Care Plan.
type CarePickupScheduleRows interface {
	scheduleModels.StudentPickupScheduleRepository
}

// CarePickupExceptionRows serve the retained pickup exceptions from Care
// Plan.
type CarePickupExceptionRows interface {
	scheduleModels.StudentPickupExceptionRepository
}

// CarePickupNoteRows serve the retained pickup notes from Care Plan.
type CarePickupNoteRows interface {
	scheduleModels.StudentPickupNoteRepository
}

// CareScheduleRequestRows serve the retained care schedule change requests
// from Care Plan.
type CareScheduleRequestRows interface {
	scheduleModels.CareScheduleChangeRequestRepository
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
	Delete(ctx context.Context, id any) error
	FindByDateRange(ctx context.Context, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error)
	FindByID(ctx context.Context, id any) (*scheduleModels.StaffShift, error)
	FindByStaffAndDateRange(ctx context.Context, staffID int64, start, end scheduleModels.Date) ([]*scheduleModels.StaffShift, error)
	FindByStaffIDsAndDates(ctx context.Context, staffIDs []int64, dates []scheduleModels.Date) ([]*scheduleModels.StaffShift, error)
	FindUsedCalendarWeeks(ctx context.Context, start, end scheduleModels.Date) ([]scheduleModels.Date, error)
	Update(ctx context.Context, shift *scheduleModels.StaffShift) error
}

// WorkforceShiftTypeRows serve the retained shift types from Workforce.
type WorkforceShiftTypeRows interface {
	Create(ctx context.Context, shiftType *scheduleModels.ShiftType) error
	Delete(ctx context.Context, id any) error
	ListAll(ctx context.Context) ([]*scheduleModels.ShiftType, error)
}
