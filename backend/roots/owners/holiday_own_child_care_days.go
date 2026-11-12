package owners

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-ogs/project-phoenix/modules/careplan/compose"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// NewHolidayOwnChildPlans binds the dated exceptions and the bulk command
// Enrollment's own-child reconciliation consumes (#6678). The booking is the
// author of a planned day (source holiday_booking, no staff member), as for
// the days of a holiday guest (#6730).
func NewHolidayOwnChildPlans(care HolidayGuestCarePlan) enrollment.HolidayOwnChildPlans {
	if care == nil {
		panic("holiday own-child plans: Care Plan is required")
	}
	return holidayOwnChildPlans{care: care, batch: careplanCompose.NewExceptionBatch(func(careplanCompose.Observation) {})}
}

type holidayOwnChildPlans struct {
	care  HolidayGuestCarePlan
	batch careplan.ExceptionBatchCommands
}

func (p holidayOwnChildPlans) HolidayOwnChildArrivals(ctx context.Context, from calendar.Date) ([]enrollment.HolidayOwnChildException, error) {
	rows, err := p.care.ListArrivalExceptions(ctx, careplan.StudentScheduleFilter{From: careplan.Date(from)})
	if err != nil {
		return nil, err
	}
	result := make([]enrollment.HolidayOwnChildException, 0, len(rows))
	for _, row := range rows {
		result = append(result, enrollment.HolidayOwnChildException{ID: row.ID, StudentID: row.StudentID, Day: calendar.Date(row.ExceptionDate), Reason: row.Reason})
	}
	return result, nil
}

func (p holidayOwnChildPlans) HolidayOwnChildPickups(ctx context.Context, from calendar.Date) ([]enrollment.HolidayOwnChildException, error) {
	rows, err := p.care.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{From: careplan.Date(from)})
	if err != nil {
		return nil, err
	}
	result := make([]enrollment.HolidayOwnChildException, 0, len(rows))
	for _, row := range rows {
		result = append(result, enrollment.HolidayOwnChildException{ID: row.ID, StudentID: row.StudentID, Day: calendar.Date(row.ExceptionDate), Reason: row.Reason, Excused: row.ExcusedFrom != nil})
	}
	return result, nil
}

func (p holidayOwnChildPlans) ApplyHolidayOwnChildPlans(ctx context.Context, changes enrollment.HolidayOwnChildPlanChanges) error {
	arrivals := make([]careplan.ArrivalException, 0, len(changes.Arrivals))
	for _, plan := range changes.Arrivals {
		arrivals = append(arrivals, careplan.ArrivalException{StudentID: plan.StudentID, ExceptionDate: careplan.Date(plan.Day), ExpectedArrival: &plan.Time, Reason: &plan.Reason, Source: careplan.ExceptionSourceHolidayBooking})
	}
	pickups := make([]careplan.PickupException, 0, len(changes.Pickups))
	for _, plan := range changes.Pickups {
		pickups = append(pickups, careplan.PickupException{StudentID: plan.StudentID, ExceptionDate: careplan.Date(plan.Day), PickupTime: &plan.Time, Reason: &plan.Reason, Source: careplan.ExceptionSourceHolidayBooking})
	}
	return p.batch.ApplyExceptionBatch(ctx, careplan.ExceptionBatch{Arrivals: arrivals, Pickups: pickups, DeleteArrivals: changes.DeleteArrivals, DeletePickups: changes.DeletePickups})
}
