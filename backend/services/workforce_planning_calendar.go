package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

// planningCalendarCapability serves workforce.PlanningCalendar from the
// retained holiday and closing-day services.
type planningCalendarCapability struct {
	holidays    schedule.HolidayService
	closingDays schedule.ClosingDayService
}

// PlanningCalendarCapability adapts the holiday and closing-day services; a
// nil service leaves its half of the calendar empty.
func PlanningCalendarCapability(holidays schedule.HolidayService, closingDays schedule.ClosingDayService) workforce.PlanningCalendar {
	return planningCalendarCapability{holidays: holidays, closingDays: closingDays}
}

func (c planningCalendarCapability) HolidaysInRange(ctx context.Context, from, to string) ([]workforce.PublicHoliday, error) {
	if c.holidays == nil {
		return []workforce.PublicHoliday{}, nil
	}
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	holidays, err := c.holidays.HolidaysInRange(ctx, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	result := make([]workforce.PublicHoliday, 0, len(holidays))
	for _, holiday := range holidays {
		result = append(result, workforce.PublicHoliday{Date: holiday.Date.String(), Name: holiday.Name})
	}
	return result, nil
}

func (c planningCalendarCapability) ClosingDaysInRange(ctx context.Context, from, to string) ([]workforce.ClosingPeriod, error) {
	if c.closingDays == nil {
		return []workforce.ClosingPeriod{}, nil
	}
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	days, err := c.closingDays.ClosingDaysInRange(ctx, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	result := make([]workforce.ClosingPeriod, 0, len(days))
	for _, day := range days {
		if day == nil {
			continue
		}
		result = append(result, workforce.ClosingPeriod{StartDate: day.StartDate.String(), EndDate: day.EndDate.String(), Reason: day.Reason})
	}
	return result, nil
}
