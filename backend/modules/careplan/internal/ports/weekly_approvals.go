package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type WeeklyApprovalPeople interface {
	TrackCompanionChanges(context.Context) (context.Context, func() bool)
	ActingStaffID(context.Context) (int64, error)
	LockDepartureModes(context.Context, int64) (map[string][]string, error)
	SaveDepartureModes(context.Context, int64, map[string][]string) error
	AuditDepartureModes(context.Context, int64, int64) error
}

type WeeklyApprovalArrivals interface {
	GetStudentArrivalSchedules(context.Context, int64) ([]*careplan.ArrivalSchedule, error)
	DeleteStudentArrivalSchedule(context.Context, int64) error
	UpsertStudentArrivalSchedule(context.Context, *careplan.ArrivalSchedule) error
}

type WeeklyApprovalPickups interface {
	GetStudentPickupSchedules(context.Context, int64) ([]*careplan.PickupSchedule, error)
	HasBookedOfferingPickupForWeekday(context.Context, int64, int) (bool, error)
	DeleteStudentPickupSchedule(context.Context, int64) error
	UpsertStudentPickupSchedule(context.Context, *careplan.PickupSchedule) error
}
