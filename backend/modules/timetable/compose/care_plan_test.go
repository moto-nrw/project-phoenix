package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Category and target-cohort tests must not query attendance exceptions.
type unusedCarePlanDirectory struct{}

func (unusedCarePlanDirectory) FindPickupException(context.Context, int64) (*timetable.PickupException, error) {
	return nil, errors.New("unexpected Care Plan pickup query")
}

func (unusedCarePlanDirectory) ListPickupExceptions(context.Context, timetable.PickupExceptionFilter) ([]timetable.PickupException, error) {
	return nil, errors.New("unexpected Care Plan pickup list")
}

func (unusedCarePlanDirectory) FindStudentStatusDay(context.Context, int64, bool) (*timetable.StudentStatusDay, error) {
	return nil, errors.New("unexpected Care Plan status query")
}

func (unusedCarePlanDirectory) ListStudentStatusDays(context.Context, timetable.StudentStatusDayFilter) ([]timetable.StudentStatusDay, error) {
	return nil, errors.New("unexpected Care Plan status list")
}
