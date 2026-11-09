//go:build testhooks

package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/traegercalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// TraegerTestCalendarSchools names fixed schools of a Träger with their
// federal state, for the Träger calendar's route tests.
type TraegerTestCalendarSchools []traegercalendar.SchoolState

func (s TraegerTestCalendarSchools) TraegerSchools(context.Context, int64) ([]traegercalendar.SchoolState, error) {
	return s, nil
}
