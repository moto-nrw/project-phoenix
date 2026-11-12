package owners

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar/traegercalendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// NewTraegerClosingDayHolidayNotices binds School Calendar's notices of a
// saved Träger closing day (#6434) to Enrollment's holiday closures, in the
// caller's Träger transaction. It names Enrollment's refusals in the Träger
// calendar contract. The notice follows the write of the closing day, so a
// refused notice marks the Träger transaction for rollback: the refusal is a
// 4xx, which would otherwise commit the closing day without its notice.
func NewTraegerClosingDayHolidayNotices(closures enrollment.HolidayClosures) traegercalendar.HolidayNotices {
	if closures == nil {
		panic("Träger closing day holiday notices: Enrollment's holiday closures are required")
	}
	return traegerClosingDayHolidayNotices{closures: closures}
}

type traegerClosingDayHolidayNotices struct {
	closures enrollment.HolidayClosures
}

func (n traegerClosingDayHolidayNotices) NotifyHolidayBookings(ctx context.Context, organizationID int64, day traegercalendar.ClosingDayInput) (int, error) {
	closure := enrollment.HolidayClosure{
		SchoolIDs: day.SchoolIDs, StartsOn: day.StartDate, EndsOn: day.EndDate, Reason: day.Reason,
		TraegerClosingDayID: day.ClosingDayID, Deleted: day.Deleted,
	}
	if day.Previous != nil {
		closure.Previous = &enrollment.HolidayClosureSpan{SchoolIDs: day.Previous.SchoolIDs, StartsOn: day.Previous.StartDate, EndsOn: day.Previous.EndDate}
	}
	notice, err := n.closures.NotifyHolidayClosure(ctx, organizationID, closure, day.HolidayBookingsToken)
	if err != nil {
		tenant.MarkRollback(ctx)
	}
	switch {
	case errors.Is(err, enrollment.ErrHolidayClosureChanged):
		return 0, traegercalendar.ErrHolidayBookingsChanged
	case errors.Is(err, enrollment.ErrHolidayHostSchoolOutsideTraeger):
		return 0, traegercalendar.ErrSchoolOutside
	case err != nil:
		return 0, err
	}
	return notice.Families, nil
}
