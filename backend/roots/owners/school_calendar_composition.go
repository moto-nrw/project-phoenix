package owners

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// planExportClosingDays and planExportHolidays serve the plan export's
// non-working-day labels from the School Calendar.
type planExportClosingDays struct{ calendar schoolcalendar.Days }

func (a planExportClosingDays) ClosingDaysInRange(ctx context.Context, from, to planexport.Date) ([]*planexport.ClosingPeriod, error) {
	if _, _, err := ParseCapabilityRange(string(from), string(to)); err != nil {
		return nil, fmt.Errorf("plan export range: %w", err)
	}
	ranges, err := a.calendar.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: string(from), OverlappingTo: string(to)})
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.ClosingPeriod, 0, len(ranges))
	for _, closing := range ranges {
		out = append(out, &planexport.ClosingPeriod{
			StartDate: planexport.Date(closing.StartDate), EndDate: planexport.Date(closing.EndDate), Reason: closing.Reason,
		})
	}
	return out, nil
}

type planExportHolidays struct{ calendar schoolcalendar.Days }

func (a planExportHolidays) HolidaysInRange(ctx context.Context, from, to planexport.Date) ([]planexport.Holiday, error) {
	if _, _, err := ParseCapabilityRange(string(from), string(to)); err != nil {
		return nil, fmt.Errorf("plan export range: %w", err)
	}
	holidays, err := a.calendar.TenantHolidays(ctx, string(from), string(to))
	if err != nil {
		return nil, err
	}
	out := make([]planexport.Holiday, 0, len(holidays))
	for _, holiday := range holidays {
		out = append(out, planexport.Holiday{Date: planexport.Date(holiday.Date), Name: holiday.Name})
	}
	return out, nil
}
