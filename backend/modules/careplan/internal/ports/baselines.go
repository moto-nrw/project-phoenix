package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type BookingBaselineSource interface {
	BookingsAuthoritative(context.Context) (bool, error)
	BookingRows(context.Context, []int64, calendar.Date, calendar.Date) ([]*careplan.ApprovedBooking, error)
	OfferingRows(context.Context, []*careplan.ApprovedBooking) (map[int64]*careplan.CareOffering, error)
}
type ArrivalBaselineSource interface {
	BookingBaselineSource
	StoredArrivalRows(context.Context, []int64) ([]*careplan.ArrivalSchedule, error)
	StudentClasses(context.Context, []int64) (map[int64]string, error)
	ClassArrivalTimes(context.Context, map[int64]string) (map[string]*domain.ClassArrivalBaseline, error)
	ClassArrivalExceptions(context.Context, map[int64]string, calendar.Date, calendar.Date) (map[string]careplan.ClassArrivalExceptionsByDate, error)
}
type PickupBaselineSource interface {
	BookingBaselineSource
	StoredPickupRows(context.Context, []int64) ([]*careplan.PickupSchedule, error)
}
