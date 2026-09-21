package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type RequestDiffPeople interface {
	DepartureModesForStudent(context.Context, int64) (map[string][]string, error)
}

type RequestDiffArrivals interface {
	GetStudentArrivalSchedules(context.Context, int64) ([]*careplan.ArrivalSchedule, error)
}

type RequestDiffPickups interface {
	GetStudentPickupSchedules(context.Context, int64) ([]*careplan.PickupSchedule, error)
	ExceptionPickupTime(context.Context, int64, careplan.Date) (*time.Time, error)
	EffectivePickupTime(context.Context, int64, careplan.Date) (*time.Time, error)
}
