//go:build testhooks

package owners

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// HolidayTestSchoolCalendar answers the same days without care for every
// host school (#4702); the zero value has none, so a test of another rule
// books every week.
type HolidayTestSchoolCalendar []enrollment.HolidayNoCareDay

func (c HolidayTestSchoolCalendar) HolidayNoCareDays(_ context.Context, schoolIDs []int64, from, to calendar.Date) (map[int64][]enrollment.HolidayNoCareDay, error) {
	result := make(map[int64][]enrollment.HolidayNoCareDay, len(schoolIDs))
	for _, schoolID := range schoolIDs {
		for _, day := range c {
			if !day.Date.Before(from) && !day.Date.After(to) {
				result[schoolID] = append(result[schoolID], day)
			}
		}
	}
	return result, nil
}
