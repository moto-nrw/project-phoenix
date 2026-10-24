package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type CareExceptions interface{ care.CareExceptions }
type CareExceptionsCallbacks struct {
	LockStudentAndExceptionDay func(ctx context.Context, id int64, date string) error
	ListPickupExceptions       func(ctx context.Context, filter StudentScheduleFilterSnapshot) ([]PickupExceptionSnapshot, error)
	ListArrivalExceptions      func(ctx context.Context, filter StudentScheduleFilterSnapshot) ([]ArrivalExceptionSnapshot, error)
	ListDepartureExceptions    func(ctx context.Context, filter StudentScheduleFilterSnapshot) ([]DepartureExceptionSnapshot, error)
}

func NewCareExceptions(callbacks CareExceptionsCallbacks) CareExceptions {
	return careExceptions{callbacks}
}

type careExceptions struct{ callbacks CareExceptionsCallbacks }

func (a careExceptions) LockStudentAndExceptionDay(ctx context.Context, id int64, date string) error {
	return a.callbacks.LockStudentAndExceptionDay(ctx, id, date)
}
func (a careExceptions) ListPickupExceptions(ctx context.Context, filter care.StudentScheduleFilter) ([]care.PickupException, error) {
	return a.callbacks.ListPickupExceptions(ctx, filter)
}
func (a careExceptions) ListArrivalExceptions(ctx context.Context, filter care.StudentScheduleFilter) ([]care.ArrivalException, error) {
	return a.callbacks.ListArrivalExceptions(ctx, filter)
}
func (a careExceptions) ListDepartureExceptions(ctx context.Context, filter care.StudentScheduleFilter) ([]care.DepartureException, error) {
	return a.callbacks.ListDepartureExceptions(ctx, filter)
}
