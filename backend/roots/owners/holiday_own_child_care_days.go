package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewHolidayOwnChildPlans binds the dated exceptions, bulk command and staff
// accounts Enrollment's own-child reconciliation consumes.
func NewHolidayOwnChildPlans(care HolidayGuestCarePlan, staff HolidayGuestStaff) (enrollment.HolidayOwnChildPlans, enrollment.HolidayOwnChildAuthors) {
	if care == nil || staff == nil {
		panic("holiday own-child plans: Care Plan and staff are required")
	}
	return holidayOwnChildPlans{care: care, batch: careplanCompose.NewExceptionBatch(func(careplanCompose.Observation) {})}, holidayOwnChildAuthors{staff: staff.StaffRows()}
}

type holidayOwnChildPlans struct {
	care  HolidayGuestCarePlan
	batch careplan.ExceptionBatchCommands
}

type holidayOwnChildAuthors struct{ staff peoplePorts.StaffRepository }

func (a holidayOwnChildAuthors) HolidayOwnChildAuthors(ctx context.Context) (map[int64]int64, error) {
	members, err := a.staff.ListMembershipWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	authors := make(map[int64]int64, len(members))
	for _, member := range members {
		if member.Person != nil && member.Person.AccountID != nil {
			authors[*member.Person.AccountID] = member.ID
		}
	}
	return authors, nil
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
		arrivals = append(arrivals, careplan.ArrivalException{StudentID: plan.StudentID, ExceptionDate: careplan.Date(plan.Day), ExpectedArrival: &plan.Time, Reason: &plan.Reason, CreatedBy: plan.StaffID, Source: careplan.ExceptionSourceStaff})
	}
	pickups := make([]careplan.PickupException, 0, len(changes.Pickups))
	for _, plan := range changes.Pickups {
		pickups = append(pickups, careplan.PickupException{StudentID: plan.StudentID, ExceptionDate: careplan.Date(plan.Day), PickupTime: &plan.Time, Reason: &plan.Reason, CreatedBy: plan.StaffID, Source: careplan.ExceptionSourceStaff})
	}
	return p.batch.ApplyExceptionBatch(ctx, careplan.ExceptionBatch{Arrivals: arrivals, Pickups: pickups, DeleteArrivals: changes.DeleteArrivals, DeletePickups: changes.DeletePickups})
}
