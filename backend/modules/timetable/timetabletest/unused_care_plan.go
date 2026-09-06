package timetabletest

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The default fixture exercises Timetable without cross-owner exceptions.
// Cross-owner tests construct their real Care Plan query at the root.
type unusedCarePlanQueries struct{}

func (unusedCarePlanQueries) FindPickupException(context.Context, int64) (*timetable.PickupException, error) {
	return nil, errors.New("test Timetable: Care Plan pickup query was not supplied")
}

func (unusedCarePlanQueries) ListPickupExceptions(context.Context, timetable.PickupExceptionFilter) ([]timetable.PickupException, error) {
	return nil, errors.New("test Timetable: Care Plan pickup list was not supplied")
}

func (unusedCarePlanQueries) FindStudentStatusDay(context.Context, int64, bool) (*timetable.StudentStatusDay, error) {
	return nil, errors.New("test Timetable: Care Plan status query was not supplied")
}

func (unusedCarePlanQueries) ListStudentStatusDays(context.Context, timetable.StudentStatusDayFilter) ([]timetable.StudentStatusDay, error) {
	return nil, errors.New("test Timetable: Care Plan status list was not supplied")
}
