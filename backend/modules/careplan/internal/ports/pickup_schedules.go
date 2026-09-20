package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PickupScheduleRepository interface {
	EffectiveScheduleRepository[*careplan.PickupSchedule]
	FindByID(context.Context, int64) (*careplan.PickupSchedule, error)
}

type PickupBulkStudent struct {
	careplan.ScheduleStudent
	PersonID int64
	Eligible bool
}

type PickupBulkStudents interface {
	FindByIDs(context.Context, []int64, calendar.Date) (map[int64]PickupBulkStudent, error)
	LockByID(context.Context, int64, calendar.Date) (PickupBulkStudent, error)
	Name(context.Context, PickupBulkStudent) (string, error)
}

type PickupScheduleTransaction interface {
	EffectiveTimeTransaction
	LockStudent(context.Context, int64) error
}
