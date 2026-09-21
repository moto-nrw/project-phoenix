package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type CareDayRecords interface {
	ListArrivalSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error)
	ListArrivalExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
}

type CareParticipationResolver interface {
	ParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[calendar.Date]map[int64]bool, error)
}
