package statistics

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
)

// WithHolidayCareDays lets a confirmed holiday booking decide the Soll on
// the days it covers (#5994, ADR 0156 addendum): the booked time model is
// the arrival and the pickup, whatever the contract or the care plan say on
// that weekday. There is still one Soll per child and day, never the sum of
// both. A cancellation of the day ("Kommt heute nicht") stays. Without
// holiday bookings the Soll beneath decides alone.
func WithHolidayCareDays(plan ports.PlannedCareDays, holidays ports.HolidayCareDays) ports.PlannedCareDays {
	if plan == nil || holidays == nil {
		return plan
	}
	return holidayCareDays{plan: plan, holidays: holidays}
}

type holidayCareDays struct {
	plan     ports.PlannedCareDays
	holidays ports.HolidayCareDays
}

func (h holidayCareDays) PlannedCareDays(ctx context.Context, studentIDs []int64, from, to ports.Date) (map[int64]map[ports.Date]studentpresence.PlannedCareDay, error) {
	planned, err := h.plan.PlannedCareDays(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	holidays, err := h.holidays.HolidayCareDays(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	for studentID, days := range holidays {
		if planned[studentID] == nil {
			planned[studentID] = map[ports.Date]studentpresence.PlannedCareDay{}
		}
		for date, booked := range days {
			planned[studentID][date] = holidayCareDay(booked, planned[studentID][date])
		}
	}
	return planned, nil
}

// holidayCareDay is the Soll of a day a holiday booking covers, over the
// verdict beneath it, of which only a cancellation of the day stays.
func holidayCareDay(booked studentpresence.HolidayCareDay, beneath studentpresence.PlannedCareDay) studentpresence.PlannedCareDay {
	day := studentpresence.PlannedCareDay{Source: studentpresence.PlannedCareSourceHoliday}
	if beneath.Cancelled {
		day.Cancelled = true
		return day
	}
	if booked.Ambiguous {
		// The booking fits two or more children: no Soll, so the day is
		// never compared, neither against the booking nor the contract.
		day.Expected, day.NoPlan = true, true
		return day
	}
	arrival, pickup := booked.Arrival, booked.Pickup
	day.Expected, day.Arrival, day.Pickup = true, &arrival, &pickup
	return day
}
