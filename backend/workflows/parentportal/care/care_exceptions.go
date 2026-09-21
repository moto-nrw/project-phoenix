package care

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// CareExceptions is the Care Plan read and lock surface of the day
// exceptions. All calls share the caller's tenant transaction; the guardian
// writes go through GuardianPickupExceptions after the student/day lock.
type CareExceptions interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	ListArrivalExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error)
}

func (s *Service) pickupExceptionForDate(ctx context.Context, id int64, date careplan.Date) (*careplan.PickupException, error) {
	rows, err := s.CareExceptions.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: date})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (s *Service) arrivalExceptionForDate(ctx context.Context, id int64, date careplan.Date) (*careplan.ArrivalException, error) {
	rows, err := s.CareExceptions.ListArrivalExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: date})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
