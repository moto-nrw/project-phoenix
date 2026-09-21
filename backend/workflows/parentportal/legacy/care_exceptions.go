package legacy

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// CareExceptions is the parent workflow's native Care Plan boundary. All calls
// share the caller's tenant transaction; writes follow the student/day lock.
type CareExceptions interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	ListArrivalExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error)
	CreatePickupException(context.Context, careplan.PickupException) (careplan.PickupException, error)
	UpdatePickupException(context.Context, careplan.PickupException) error
	DeletePickupException(context.Context, int64) error
}

func (s *service) pickupExceptionForDate(ctx context.Context, id int64, date careplan.Date) (*careplan.PickupException, error) {
	rows, err := s.CareExceptions.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: date})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (s *service) arrivalExceptionForDate(ctx context.Context, id int64, date careplan.Date) (*careplan.ArrivalException, error) {
	rows, err := s.CareExceptions.ListArrivalExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: date})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
