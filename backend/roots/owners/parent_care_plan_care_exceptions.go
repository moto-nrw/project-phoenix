package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentCareExceptions(source ParentCareExceptionSource) parentportal.CareExceptions {
	if source == nil {
		return nil
	}
	return parentportal.NewCareExceptions(parentportal.CareExceptionsCallbacks{LockStudentAndExceptionDay: func(ctx context.Context, id int64, date string) error {
		return source.LockStudentAndExceptionDay(ctx, id, date)
	},
		ListPickupExceptions: func(ctx context.Context, filter parentportal.StudentScheduleFilterSnapshot) ([]parentportal.PickupExceptionSnapshot, error) {
			value, err := source.ListPickupExceptions(ctx, parentCarePlanToOwnerStudentScheduleFilter(filter))
			return parentCarePlanSlice(value, parentCarePlanFromOwnerPickupException), err
		},
		ListArrivalExceptions: func(ctx context.Context, filter parentportal.StudentScheduleFilterSnapshot) ([]parentportal.ArrivalExceptionSnapshot, error) {
			value, err := source.ListArrivalExceptions(ctx, parentCarePlanToOwnerStudentScheduleFilter(filter))
			return parentCarePlanSlice(value, parentCarePlanFromOwnerArrivalException), err
		},
		ListDepartureExceptions: func(ctx context.Context, filter parentportal.StudentScheduleFilterSnapshot) ([]parentportal.DepartureExceptionSnapshot, error) {
			value, err := source.ListDepartureExceptions(ctx, parentCarePlanToOwnerStudentScheduleFilter(filter))
			return parentCarePlanSlice(value, parentCarePlanFromOwnerDepartureException), err
		},
	})
}

// ParentCareExceptionSource is the owner's exception read and lock facade.
type ParentCareExceptionSource interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	ListArrivalExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error)
	ListDepartureExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.DepartureException, error)
}
