package care

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareExceptions is the Care Plan read and lock surface of the day
// exceptions. All calls share the caller's tenant transaction; the guardian
// writes go through GuardianPickupExceptions after the student/day lock.

func (s *Service) departureExceptionForDate(ctx context.Context, id int64, date calendar.Date) (*DepartureException, error) {
	rows, err := s.CareExceptions.ListDepartureExceptions(ctx, StudentScheduleFilter{StudentIDs: []int64{id}, Date: date})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (s *Service) pickupExceptionForDate(ctx context.Context, id int64, date calendar.Date) (*PickupException, error) {
	rows, err := s.CareExceptions.ListPickupExceptions(ctx, StudentScheduleFilter{StudentIDs: []int64{id}, Date: date})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (s *Service) arrivalExceptionForDate(ctx context.Context, id int64, date calendar.Date) (*ArrivalException, error) {
	rows, err := s.CareExceptions.ListArrivalExceptions(ctx, StudentScheduleFilter{StudentIDs: []int64{id}, Date: date})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
